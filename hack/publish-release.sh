#!/usr/bin/env bash
# publish-release.sh — build, push, and publish a SupKube release to Azure.
#
# Single-shot MVP publisher. Does NOT touch CI — runs from the dev workstation.
# Once we've shipped 2-3 releases through this and verified the flow, we lift
# the same logic into GitHub Actions (target: v0.9.1.x).
#
# What it does
# ────────────
#   1. Read $VERSION from $1 (e.g. 0.9.0.3-alpha)
#   2. docker build backend + frontend with the version tag
#   3. docker tag images twice:
#        - supkube.azurecr.io/{backend,frontend}:$VERSION  (canonical, pushed)
#        - supkube/{backend,frontend}:$VERSION             (local shorthand for
#          existing `kubectl set image` commands on docker-desktop dev — the
#          daemon caches both refs to the same layer, so it costs nothing)
#   4. az acr login → docker push both images to ACR
#   5. Bump Chart.yaml version + appVersion to $VERSION
#   6. Bump values.yaml backend.image.tag + frontend.image.tag to $VERSION
#   7. helm package supkube-helm/supkube → dist/supkube-$VERSION.tgz
#   8. Download existing index.yaml from Blob (if present), `helm repo index
#      --merge` with --url "" so all chart entries are RELATIVE — that's what
#      lets us swap charts.supkube.com → some-other-cdn.com later without
#      touching index.yaml content
#   9. az storage blob upload .tgz + index.yaml to $web container
#  10. Print customer-facing verification commands
#
# Why relative URLs in index.yaml
# ───────────────────────────────
#   Helm clients join `helm repo add <URL>` with each entry's `urls:` field.
#   If urls are absolute (`https://supkubecharts.z13...`), the index is locked
#   to whichever URL you published with. Swapping to `charts.supkube.com` later
#   means regenerating + republishing the WHOLE index. With relative urls
#   (`supkube-0.9.0.2.tgz`), Helm resolves against whatever URL the customer
#   used — past releases stay valid forever.
#
# Usage
# ─────
#   ./hack/publish-release.sh 0.9.0.3-alpha
#
# Required env (or fill in CONFIG below)
# ──────────────────────────────────────
#   ACR_LOGIN_SERVER      — e.g. supkube.azurecr.io
#   STORAGE_ACCOUNT       — e.g. supkubecharts
#   CUSTOMER_FACING_URL   — what we tell customers to `helm repo add`. Set to
#                           https://charts.supkube.com/ once DNS is live; the
#                           script only uses this for the printed summary,
#                           index.yaml itself uses relative urls.

set -euo pipefail

# ─── CONFIG (override via env if running ad-hoc) ─────────────────────
# Values confirmed 2026-05-26 from hack/AZURE-SETUP.md walkthrough.
# ACR: supkube.azurecr.io (Standard SKU, anonymousPullEnabled=true)
# Storage: supkubecharts (Static Website endpoint below)
# RG: Research_and_Development (Sub-RnD subscription, southeastasia)
ACR_LOGIN_SERVER="${ACR_LOGIN_SERVER:-supkube.azurecr.io}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-supkubecharts}"
STORAGE_CONTAINER='$web'   # static-website container is always literally $web

# CUSTOMER_FACING_URL: the URL we tell customers to `helm repo add`.
# Backed by a Cloudflare Worker that proxies → Azure Blob Static Website.
# Worker code lives in the Cloudflare dashboard (account: Mars's Free
# plan, Worker name: charts-azure-proxy). Free plan because Cloudflare
# moved Origin Rules' Host/SNI override behind their Enterprise plan
# ($200/mo); the Worker pattern is free (100k req/day quota is ~100x
# what we'd ever hit).
#
# index.yaml uses RELATIVE chart urls so swapping the customer URL again
# (different CDN, vanity domain change, etc.) needs zero re-publish —
# nothing in the .tgz files or index.yaml is locked to this hostname.
CUSTOMER_FACING_URL="${CUSTOMER_FACING_URL:-https://charts.supkube.com/}"

CHART_DIR="supkube-helm/supkube"
DIST_DIR="dist"

# Image short names — used both for ACR fully-qualified path AND for the
# legacy supkube/{name} local-dev tag.
BACKEND_NAME="backend"
FRONTEND_NAME="frontend"

# ─── Args ────────────────────────────────────────────────────────────
VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  cat <<EOF
Usage: $0 <version>

Examples:
  $0 0.9.0.3-alpha          # alpha release
  $0 0.9.0.3                # stable release (no suffix)

The version becomes:
  - Image tag:         $ACR_LOGIN_SERVER/{backend,frontend}:<version>
  - Chart version:     <version>
  - Chart appVersion:  <version>
EOF
  exit 1
fi

# ─── Sanity: prerequisites ───────────────────────────────────────────
require() {
  command -v "$1" >/dev/null 2>&1 || { echo "ERROR: '$1' not in PATH"; exit 1; }
}
require docker
require helm
require az
require yq   # github.com/mikefarah/yq (Go version, not the Python one)

# Translate SupKube's 4-segment versioning (X.Y.Z.W-suffix) into strict
# SemVer (X.Y.Z[-pre]) for Helm's chart.version field. Helm rejects 4-segment
# versions even though they're common in vendor-internal versioning. We
# preserve the original tag everywhere customer-visible (image tag,
# appVersion) — only chart.version is translated, and only because Helm
# enforces it.
#
# Translation rules (chosen so SemVer ordering still matches release order):
#   0.9.0.2-alpha → 0.9.0-alpha.2   (4-seg + suffix → 3-seg + suffix.N)
#   0.9.0.2       → 0.9.0-2         (4-seg no suffix → 3-seg + pre.N; rare)
#   0.9.0-alpha   → 0.9.0-alpha     (already SemVer, pass through)
#   0.9.0         → 0.9.0           (stable, pass through)
chart_version_from() {
  local v="$1"
  if [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-.+)?$ ]]; then
    echo "$v"; return 0
  fi
  if [[ "$v" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)\.([0-9]+)-(.+)$ ]]; then
    echo "${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}-${BASH_REMATCH[5]}.${BASH_REMATCH[4]}"
    return 0
  fi
  if [[ "$v" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[3]}-${BASH_REMATCH[4]}"
    return 0
  fi
  echo "ERROR: cannot translate version '$v' to valid SemVer for Helm chart" >&2
  return 1
}

# Find repo root no matter where the script is invoked from.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

[[ -d "$CHART_DIR" ]] || { echo "ERROR: chart dir not found at $CHART_DIR"; exit 1; }

mkdir -p "$DIST_DIR"

# Translate user-facing 4-segment version (image tag / appVersion / what
# Mars types) into SemVer for Helm chart metadata.
CHART_VERSION=$(chart_version_from "$VERSION")

echo "════════════════════════════════════════════════════════════════"
echo "  SupKube release publisher"
echo "════════════════════════════════════════════════════════════════"
echo "  Version (image/app): $VERSION"
echo "  Chart version:       $CHART_VERSION"
echo "  ACR:                 $ACR_LOGIN_SERVER"
echo "  Storage Account:     $STORAGE_ACCOUNT"
echo "  Customer URL:        $CUSTOMER_FACING_URL"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Multi-arch via Buildx
# ─────────────────────
# We publish manifest-list images that contain BOTH linux/amd64 AND
# linux/arm64 variants. Kubernetes nodes auto-select the matching arch
# at pull time, so customers run the SAME `helm install` command
# regardless of whether they're on AKS x86, AWS Graviton arm64, or
# Apple Silicon docker-desktop. No customer-facing --platform flag,
# no per-arch tag, no "did you pick the right image?" support tickets.
#
# `docker buildx build --push` is the ONLY way to publish a multi-arch
# manifest list — the legacy `docker build` + `docker push` pair can't
# do it because the daemon only stores one arch at a time. Buildx hands
# the multi-arch artifact directly to the registry.
#
# Requires Dockerfile.backend to declare `ARG TARGETARCH` and pass it
# to `go build` as GOARCH (otherwise Go cross-compile picks the build
# host's arch for every target — silently emitting 2 identical
# binaries). See docker/Dockerfile.backend for that wiring. Frontend
# is arch-agnostic JS bundled into multi-arch nginx — no Dockerfile
# changes needed.
PLATFORMS="${SUPKUBE_BUILD_PLATFORMS:-linux/amd64,linux/arm64}"

# ─── 0/9. Housekeeping: prune stale buildx cache ─────────────────────
# Why: multi-arch buildx accumulates ~7 GB of cache per publish; three
# back-to-back publishes can wedge the SSD (we hit ENOSPC on 2026-05-27).
# `--filter until=72h` keeps the last ~3 days of cache so same-week
# publishes still benefit from incremental layer reuse, but anything
# older — which is just last week's tag we've long since pushed to ACR
# — is freed. Silent + non-fatal: if docker isn't running yet the next
# step (`az acr login`) will surface the real error.
# Skip with: SUPKUBE_SKIP_PRUNE=1 hack/publish-release.sh ...
if [[ "${SUPKUBE_SKIP_PRUNE:-0}" != "1" ]]; then
  echo "▶ [0/9] Housekeeping: pruning buildx cache older than 72h…"
  BEFORE=$(docker system df --format '{{.Size}}' 2>/dev/null | head -1 || echo "?")
  docker builder prune -af --filter "until=72h" >/dev/null 2>&1 || true
  AFTER=$(docker system df --format '{{.Size}}' 2>/dev/null | head -1 || echo "?")
  echo "  build cache: $BEFORE → $AFTER"
fi

# ─── 1/9. Setup buildx + ACR auth ────────────────────────────────────
echo "▶ [1/9] Preparing multi-arch buildx + ACR login…"
ACR_NAME="${ACR_LOGIN_SERVER%%.*}"  # strip ".azurecr.io" → just the registry name
az acr login --name "$ACR_NAME"

# Buildx builder. Modern docker-desktop (2023+) ships a multi-arch
# builder by default ("desktop-linux"), but CI runners + older setups
# need an explicit `buildx create`. Idempotent: if our named builder
# already exists, we just `use` it.
if ! docker buildx inspect supkube-multi >/dev/null 2>&1; then
  echo "  Creating buildx builder 'supkube-multi'…"
  docker buildx create --name supkube-multi --use --bootstrap
else
  docker buildx use supkube-multi
fi

# ─── 2/9. Build & push backend (multi-arch) ──────────────────────────
echo ""
echo "▶ [2/9] Building backend image (platforms: $PLATFORMS) → ACR…"
docker buildx build \
  --platform "$PLATFORMS" \
  --push \
  -f docker/Dockerfile.backend \
  -t "$ACR_LOGIN_SERVER/$BACKEND_NAME:$VERSION" \
  --build-arg "SUPKUBE_VERSION=$VERSION" \
  supkube-backend

# ─── 3/9. Build & push frontend (multi-arch) ─────────────────────────
echo ""
echo "▶ [3/9] Building frontend image (platforms: $PLATFORMS) → ACR…"
docker buildx build \
  --platform "$PLATFORMS" \
  --push \
  -f docker/Dockerfile.frontend \
  -t "$ACR_LOGIN_SERVER/$FRONTEND_NAME:$VERSION" \
  supkube-frontend

# Local-dev shorthand: after the multi-arch push, pull the native-arch
# variant back into the local Docker daemon and tag it as
# `supkube/<name>:<version>` so historical local-dev shortcuts like
# `kubectl set image deploy/supkube-backend backend=supkube/backend:VERSION`
# (against the docker-desktop cluster) still work. Buildx `--load` can't
# load multi-arch images into the daemon — but a plain `docker pull` of
# a multi-arch tag fetches just the native arch, which is what we want.
echo "  Pulling native-arch variants back into local daemon for dev shortcut…"
docker pull "$ACR_LOGIN_SERVER/$BACKEND_NAME:$VERSION" >/dev/null
docker pull "$ACR_LOGIN_SERVER/$FRONTEND_NAME:$VERSION" >/dev/null
docker tag "$ACR_LOGIN_SERVER/$BACKEND_NAME:$VERSION"  "supkube/$BACKEND_NAME:$VERSION"
docker tag "$ACR_LOGIN_SERVER/$FRONTEND_NAME:$VERSION" "supkube/$FRONTEND_NAME:$VERSION"

# ─── 4/9. Bump Chart.yaml ────────────────────────────────────────────
echo ""
echo "▶ [4/9] Bumping Chart.yaml: version=$CHART_VERSION (SemVer), appVersion=$VERSION"
yq -i ".version = \"$CHART_VERSION\"" "$CHART_DIR/Chart.yaml"
yq -i ".appVersion = \"$VERSION\"" "$CHART_DIR/Chart.yaml"

# ─── 5/9. Bump values.yaml image tags ────────────────────────────────
echo ""
echo "▶ [5/9] Bumping values.yaml image tags → $VERSION"
yq -i ".backend.image.tag = \"$VERSION\"" "$CHART_DIR/values.yaml"
yq -i ".frontend.image.tag = \"$VERSION\"" "$CHART_DIR/values.yaml"

# ─── 6/9. Package chart ──────────────────────────────────────────────
echo ""
echo "▶ [6/9] Packaging Helm chart…"
# `helm dependency update` pulls subcharts (Velero) — without this the package
# would have a broken `dependencies/` dir and `helm install` from the repo
# would fail with "chart velero not found".
helm dependency update "$CHART_DIR"
helm package "$CHART_DIR" --destination "$DIST_DIR"

CHART_TGZ="$DIST_DIR/supkube-$CHART_VERSION.tgz"
[[ -f "$CHART_TGZ" ]] || { echo "ERROR: expected chart at $CHART_TGZ — helm package produced something else?"; exit 1; }

# ─── 7/9. Merge existing index.yaml ──────────────────────────────────
echo ""
echo "▶ [7/9] Merging into existing index.yaml…"
EXISTING_INDEX="$DIST_DIR/existing-index.yaml"
rm -f "$EXISTING_INDEX"

# Use HTTPS GET (not az storage blob download) so we hit the actual public
# endpoint — verifying the static site is alive and serving as we expect.
# If 404, it's a first publish — that's fine, helm repo index handles missing
# --merge gracefully.
HTTP_CODE=$(curl -sS -o "$EXISTING_INDEX" -w "%{http_code}" "${CUSTOMER_FACING_URL}index.yaml" || echo "000")
if [[ "$HTTP_CODE" == "200" ]]; then
  echo "  Found existing index.yaml — merging."
  helm repo index "$DIST_DIR" --url "" --merge "$EXISTING_INDEX"
elif [[ "$HTTP_CODE" == "404" ]]; then
  echo "  No existing index.yaml (first publish) — creating fresh."
  rm -f "$EXISTING_INDEX"
  helm repo index "$DIST_DIR" --url ""
else
  echo "ERROR: unexpected HTTP $HTTP_CODE fetching existing index.yaml from $CUSTOMER_FACING_URL"
  echo "  Possible causes: Cloudflare not yet live, DNS not propagated, storage account name wrong."
  echo "  Inspect: curl -v ${CUSTOMER_FACING_URL}index.yaml"
  exit 1
fi

# Sanity check: index.yaml entries should be relative paths, NOT absolute.
# If --url "" silently fell through to a default, abort — we'd publish
# poison that locks future releases to the current URL.
if grep -E "^\s+-\s+https?://" "$DIST_DIR/index.yaml" >/dev/null; then
  echo "ERROR: index.yaml contains absolute URLs — relative-URL invariant broken."
  echo "  Inspect: cat $DIST_DIR/index.yaml"
  exit 1
fi

# ─── 8/9. Upload to Blob ─────────────────────────────────────────────
echo ""
echo "▶ [8/9] Uploading to $STORAGE_ACCOUNT/\$web…"

# Auth mode: prefer login (AAD), fall back to account key.
# Why both: AAD/RBAC is the secure default but suffers from 1-10 min
# propagation lag the first time a user/role is added — on a fresh
# workstation a publish run would fail with 403 and the user has no
# idea why. Account key auth is instant and the same person (the dev
# running this script) already has Owner on the storage account, so
# fetching the key is allowed. We try login first; if it 403s, drop to
# key auth so a publish never gets blocked by RBAC propagation.
upload_blob() {
  local name="$1" file="$2" cache="$3" content_type="${4:-}"
  local args=(
    --account-name "$STORAGE_ACCOUNT"
    --container-name "$STORAGE_CONTAINER"
    --name "$name"
    --file "$file"
    --content-cache "$cache"
    --overwrite
  )
  [[ -n "$content_type" ]] && args+=(--content-type "$content_type")

  # Attempt 1: AAD/RBAC (preferred — leaves no static credentials in env).
  if az storage blob upload --auth-mode login "${args[@]}" 2>/dev/null; then
    return 0
  fi

  echo "  (login auth failed, falling back to account key — likely RBAC propagation lag)"
  if [[ -z "${AZURE_STORAGE_KEY:-}" ]]; then
    AZURE_STORAGE_KEY=$(az storage account keys list \
      --account-name "$STORAGE_ACCOUNT" --query '[0].value' -o tsv)
    export AZURE_STORAGE_KEY
  fi
  az storage blob upload --auth-mode key --account-key "$AZURE_STORAGE_KEY" "${args[@]}"
}

# .tgz: immutable forever (versioned filename), aggressive cache.
upload_blob "supkube-$CHART_VERSION.tgz" "$CHART_TGZ" "public, max-age=31536000, immutable"

# index.yaml: mutable lookup table, NO cache (otherwise customers can't
# see the latest version we just published).
upload_blob "index.yaml" "$DIST_DIR/index.yaml" \
  "no-cache, no-store, must-revalidate, max-age=0" "application/yaml"

# preflight.sh: customer-facing curl-bash compatibility checker. Tracks the
# script in hack/preflight.sh — publish-release.sh keeps the public copy
# in sync on every release. No-cache so an updated script (e.g. new check
# added) reaches existing customers immediately.
PREFLIGHT_SRC="$REPO_ROOT/hack/preflight.sh"
if [[ -f "$PREFLIGHT_SRC" ]]; then
  echo "  Uploading preflight.sh (customer pre-install checker)…"
  upload_blob "preflight.sh" "$PREFLIGHT_SRC" \
    "no-cache, no-store, must-revalidate, max-age=0" "text/x-shellscript"
fi

# supkube_debug.sh: customer-facing diagnostic bundle generator (v0.8.14
# LV6). Same no-cache strategy as preflight.sh — updating the script (new
# CR collected, new error pattern recognized) reaches the customer base on
# the next invocation. Customer runs:
#   curl -fsSL https://charts.supkube.com/supkube_debug.sh | bash -s -- -n supkube -o debug.tar.gz
DEBUG_SRC="$REPO_ROOT/hack/supkube_debug.sh"
if [[ -f "$DEBUG_SRC" ]]; then
  echo "  Uploading supkube_debug.sh (diagnostic bundle generator)…"
  upload_blob "supkube_debug.sh" "$DEBUG_SRC" \
    "no-cache, no-store, must-revalidate, max-age=0" "text/x-shellscript"
fi

# ─── 9/9. Verify + print summary ─────────────────────────────────────
echo ""
echo "▶ [9/9] Verifying public reachability…"

# Cloudflare may cache the old index briefly — purge would be cleanest but
# requires an API token. For now we just verify the new version IS in
# whatever the public endpoint serves; if Cloudflare is stale, this fails
# loud and Mars can purge the URL manually.
sleep 5  # give the upload a moment to propagate through Cloudflare's edge
SERVED=$(curl -sS "${CUSTOMER_FACING_URL}index.yaml" | grep -c "^\s*version: $CHART_VERSION$" || true)
if [[ "$SERVED" -lt 1 ]]; then
  echo "WARN: $CUSTOMER_FACING_URL/index.yaml does not yet contain chart version $CHART_VERSION."
  echo "  Cloudflare may be serving a cached copy. Purge from Cloudflare dashboard:"
  echo "    Caching → Configuration → Purge Everything"
  echo "  Or wait 5 minutes for the default edge TTL to expire."
else
  echo "  ✓ $CUSTOMER_FACING_URL/index.yaml now serves chart version $CHART_VERSION"
fi

cat <<EOF

════════════════════════════════════════════════════════════════
  ✅ Published SupKube $VERSION (chart $CHART_VERSION)
════════════════════════════════════════════════════════════════

Customer-facing install command (now actually works):

  # Recommended: run preflight first (10 checks against the target cluster)
  curl -fsSL ${CUSTOMER_FACING_URL}preflight.sh | bash

  helm repo add supkube $CUSTOMER_FACING_URL
  helm repo update
  helm install supkube supkube/supkube --version $CHART_VERSION --devel \\
    -n supkube --create-namespace \\
    --set eula.accept=true \\
    --set velero.enabled=true

Verify history:

  helm search repo supkube --versions

Images live at:

  $ACR_LOGIN_SERVER/$BACKEND_NAME:$VERSION
  $ACR_LOGIN_SERVER/$FRONTEND_NAME:$VERSION

Don't forget:

  git add $CHART_DIR/Chart.yaml $CHART_DIR/values.yaml
  git commit -m "release: $VERSION"
  git tag v$VERSION
  git push && git push --tags

EOF
