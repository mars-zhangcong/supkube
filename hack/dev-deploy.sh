#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════════════╗
# ║  ⚠⚠⚠  DEPRECATED (2026-06-01, ADR-042)  ⚠⚠⚠                              ║
# ║                                                                          ║
# ║  本脚本已退役。新流程见 .github/workflows/cd.yaml + 架构设计.md ADR-042：  ║
# ║    - push to main          → 自动推 aks-jumborca-dev (Mars 日常)         ║
# ║    - tag v*                → 推 aks-jumborca-test                        ║
# ║    - manual approval       → 推 aks-jumborca-prod                        ║
# ║                                                                          ║
# ║  本机不再需要 docker-desktop (已关). 所有部署走 CI/CD 闭环。              ║
# ║                                                                          ║
# ║  为何还留这个文件:                                                        ║
# ║    1. 历史 git blame 参考 (60+ commits 的踩坑总结都在注释里)             ║
# ║    2. CI 完全瘫痪时的应急 fallback                                       ║
# ║    3. ADR-042 §6 "渐进退役" — 下个 sprint 在 v0.9.1.15 真正删除          ║
# ║                                                                          ║
# ║  ⛔ 不要日常使用. 用 `git push` 触发 cd.yaml 才是新流程 (ADR-042)。       ║
# ║                                                                          ║
# ║  ──────────────────── (legacy header preserved below) ──────────────────  ║
# ║  dev-deploy.sh — fast, end-to-end SupKube dev iteration loop             ║
# ║                                                                          ║
# ║  build (multi-arch) → push (ACR for AKS, local daemon for docker-       ║
# ║  desktop) → kubectl set image on BOTH clusters → 4-step verification.   ║
# ║                                                                          ║
# ║  Why this script exists (the "5x false-complete" pattern)               ║
# ║  ─────────────────────────────────────────────────────────              ║
# ║  Over a recent 5-iteration stretch we kept declaring a change "done"    ║
# ║  after the docker build succeeded, only to discover the deployment      ║
# ║  was still serving stale bits. Root causes ranged from:                 ║
# ║    - ACR token silently expiring mid-push (1h TTL)                      ║
# ║    - wrong container name in `kubectl set image` (silent no-op)         ║
# ║    - only one of two clusters was updated (docker-desktop vs AKS)       ║
# ║    - frontend chunk hash mismatch (nginx still served old assets)       ║
# ║    - ServiceAccount RBAC silently dropped after a Helm upgrade (#79)    ║
# ║                                                                         ║
# ║  Each of those slipped past `docker build OK` and any visual check of  ║
# ║  the running app. The cure: 4 hard verifications that the new bits     ║
# ║  are LIVE in BOTH clusters and the SA can still list pods. Fail-fast   ║
# ║  on any deviation. No "looks fine" exit paths.                         ║
# ║                                                                         ║
# ║  Not a replacement for publish-release.sh                              ║
# ║  ───────────────────────────────────────                              ║
# ║  publish-release.sh does the prod path: helm-package, Chart.yaml bump, ║
# ║  values.yaml bump, blob upload, customer-facing index merge. That's    ║
# ║  3-5 min per iteration and is overkill for "tweak a handler, see if it ║
# ║  works". This script SKIPS chart bumps entirely — it just rolls the    ║
# ║  image on the two existing deployments. Iteration time target: ~60s    ║
# ║  backend-only, ~90s frontend-only, ~2min both.                         ║
# ║                                                                         ║
# ║  Usage                                                                 ║
# ║  ─────                                                                 ║
# ║    ./hack/dev-deploy.sh                  # build both, deploy 2 clusters║
# ║    ./hack/dev-deploy.sh --backend-only   # only backend                ║
# ║    ./hack/dev-deploy.sh --frontend-only  # only frontend               ║
# ║    ./hack/dev-deploy.sh --auto           # detect changed dir via git  ║
# ║    ./hack/dev-deploy.sh --dry-run        # print every cmd, do nothing ║
# ║    ./hack/dev-deploy.sh --no-aks         # docker-desktop only         ║
# ║    ./hack/dev-deploy.sh --skip-verify    # SKIP Phase 5 (NOT recommend)║
# ║                                                                         ║
# ║  Exit codes                                                            ║
# ║  ──────────                                                            ║
# ║    0   success, all verifications passed                               ║
# ║    1   pre-flight failed (auth, contexts, missing tools)              ║
# ║    2   build failed                                                    ║
# ║    3   deploy failed (rollout did not converge)                        ║
# ║    4   verification failed (image tag / buildStamp / asset / RBAC)     ║
# ╚══════════════════════════════════════════════════════════════════════════╝

set -euo pipefail

# ════════════════════════════════════════════════════════════════════════════
# CONFIG — override via env if Mars's setup ever diverges
# ════════════════════════════════════════════════════════════════════════════
# These mirror publish-release.sh's CONFIG so a future refactor can lift the
# shared block out. Keep them in sync if either file changes.
ACR_LOGIN_SERVER="${ACR_LOGIN_SERVER:-supkube.azurecr.io}"
ACR_NAME="${ACR_NAME:-${ACR_LOGIN_SERVER%%.*}}"   # "supkube" from "supkube.azurecr.io"

# kubectl contexts. If Mars has renamed these, override via env.
# `docker-desktop` is the literal context shipped by Docker Desktop;
# `aks-jumborca-dev` is from `az aks get-credentials -n aks-jumborca-dev ...`.
CTX_LOCAL="${CTX_LOCAL:-docker-desktop}"
CTX_AKS="${CTX_AKS:-aks-jumborca-dev}"

# Namespaces. SupKube ships in `supkube`; Velero in `velero`. Phase 5 (RBAC)
# pokes both, because issue #79 hit when the SA lost cross-namespace pod-list
# permissions silently after a chart upgrade.
NS_SUPKUBE="${NS_SUPKUBE:-supkube}"
NS_VELERO="${NS_VELERO:-velero}"

# Deployment names + container names INSIDE the pods. The container name is
# the subtle bit: the deployment is `supkube-backend` but the container
# inside it is just `backend` (same for frontend). `kubectl set image` needs
# the container name, not the deployment name — getting it wrong is a silent
# no-op (kubectl prints "deployment unchanged" but no rollout happens).
# We probe the live deployment for the real name in Phase 4 instead of
# trusting these constants, but they're here as a fallback / documentation.
DEPLOY_BACKEND="supkube-backend"
DEPLOY_FRONTEND="supkube-frontend"
CONTAINER_BACKEND_FALLBACK="backend"
CONTAINER_FRONTEND_FALLBACK="frontend"

# Repo layout
BACKEND_DIR="supkube-backend"
FRONTEND_DIR="supkube-frontend"
BACKEND_IMAGE_NAME="backend"
FRONTEND_IMAGE_NAME="frontend"

# Buildx builder name. Matches publish-release.sh so we share its cache.
BUILDX_BUILDER="supkube-multi"

# Status endpoint that returns {version, buildStamp, ...} — used by Phase 5
# step 2 ("buildStamp truly new"). If the path ever changes, update here.
STATUS_PATH="/api/v1/status"

# Temp files (cleaned by EXIT trap below).
TMP_VERSION_FILE="/tmp/supkube-dev-version"
TMP_DIR="$(mktemp -d -t supkube-dev-deploy.XXXXXX)"

# ════════════════════════════════════════════════════════════════════════════
# Flags
# ════════════════════════════════════════════════════════════════════════════
DO_BACKEND=1
DO_FRONTEND=1
DRY_RUN=0
NO_AKS=0
SKIP_VERIFY=0
AUTO_DETECT=0

usage() {
  sed -n '2,/^# ╚/p' "$0" | sed 's/^# //; s/^#//'
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backend-only)   DO_BACKEND=1; DO_FRONTEND=0 ;;
    --frontend-only)  DO_BACKEND=0; DO_FRONTEND=1 ;;
    --auto)           AUTO_DETECT=1 ;;
    --dry-run)        DRY_RUN=1 ;;
    --no-aks)         NO_AKS=1 ;;
    --skip-verify)    SKIP_VERIFY=1 ;;
    -h|--help)        usage ;;
    *) echo "ERROR: unknown flag '$1'. See --help." >&2; exit 1 ;;
  esac
  shift
done

# ════════════════════════════════════════════════════════════════════════════
# Output helpers (color, banners). Mirrors verify-cluster.sh's palette so a
# Mars-eyeball used to one will recognise the other.
# ════════════════════════════════════════════════════════════════════════════
if [[ -t 1 ]]; then
  C_OK="\033[32m"; C_WARN="\033[33m"; C_ERR="\033[31m"; C_DIM="\033[2m"
  C_BOLD="\033[1m"; C_CYAN="\033[36m"; C_OFF="\033[0m"
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_BOLD=""; C_CYAN=""; C_OFF=""
fi

ok()   { echo -e "  ${C_OK}✅${C_OFF} $*"; }
warn() { echo -e "  ${C_WARN}⚠${C_OFF}  $*"; }
err()  { echo -e "  ${C_ERR}❌${C_OFF} $*"; }
info() { echo -e "  ${C_DIM}·${C_OFF}  $*"; }

banner() {
  echo ""
  echo -e "${C_BOLD}${C_CYAN}════════════════════════════════════════════════════════════════${C_OFF}"
  echo -e "${C_BOLD}${C_CYAN}  $*${C_OFF}"
  echo -e "${C_BOLD}${C_CYAN}════════════════════════════════════════════════════════════════${C_OFF}"
}

# fatal <exit_code> <message> [recovery hint...]
# Centralised failure path so every error tells Mars HOW to recover, not just
# WHAT broke. The script's whole reason to exist is reducing time-to-diagnose,
# so "ERROR: foo failed" with no follow-up is failure mode #1 to avoid.
fatal() {
  local code="$1"; shift
  echo "" >&2
  err "$1"; shift
  while [[ $# -gt 0 ]]; do
    echo -e "    ${C_DIM}→${C_OFF} $1" >&2
    shift
  done
  exit "$code"
}

# run <cmd...> — wrap a command so --dry-run prints it instead of running it.
# Note: do NOT use this for commands whose output we capture into a variable
# (cmd-substitution); for those, just call directly and gate on $DRY_RUN at
# the call site. `run` is for side-effecting commands (build, push, set
# image, rollout) where the OUTPUT is just streamed to the terminal.
run() {
  if [[ "$DRY_RUN" == "1" ]]; then
    echo -e "  ${C_DIM}[dry-run]${C_OFF} $*"
    return 0
  fi
  "$@"
}

# Cleanup on any exit path. EXIT trap (not ERR) so it fires on success too.
cleanup() {
  rm -f "$TMP_VERSION_FILE"
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

# Repo root from script location — lets Mars invoke from anywhere.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ════════════════════════════════════════════════════════════════════════════
# PHASE 0 — Pre-flight check
# ════════════════════════════════════════════════════════════════════════════
# Goal: catch EVERY failure mode that would otherwise surface 90 seconds into
# the run (after build completes but before push/deploy). Each check is cheap
# (<1s) and the error messages include the exact recovery command.
banner "Phase 0 — Pre-flight"

PREFLIGHT_PASS=0
PREFLIGHT_TOTAL=0
PREFLIGHT_FAIL_REASONS=()

check() {
  local name="$1"; shift
  PREFLIGHT_TOTAL=$((PREFLIGHT_TOTAL + 1))
  if "$@"; then
    ok "$name"
    PREFLIGHT_PASS=$((PREFLIGHT_PASS + 1))
  else
    err "$name"
    PREFLIGHT_FAIL_REASONS+=("$name")
  fi
}

# 0.1 Tools in PATH. We list them all up front so a fresh workstation gets
#     one consolidated error instead of failing piecemeal.
check_tool() { command -v "$1" >/dev/null 2>&1; }

check "docker on PATH"  check_tool docker
check "kubectl on PATH" check_tool kubectl
check "az on PATH"      check_tool az
check "jq on PATH"      check_tool jq
if [[ "$DO_BACKEND" == "1" ]];  then check "go on PATH (backend build)"  check_tool go; fi
if [[ "$DO_FRONTEND" == "1" ]]; then check "npm on PATH (frontend build)" check_tool npm; fi

# 0.2 Docker buildx available. publish-release.sh assumes it; we do too.
check "docker buildx available" bash -c 'docker buildx version >/dev/null 2>&1'

# 0.3 az has a live token. `az account show` returns non-zero if not logged
#     in — much faster than trying `az acr login` and parsing its error.
check "az logged in" bash -c 'az account show >/dev/null 2>&1'

# 0.4 ACR token fresh. The 1-hour TTL is the single biggest source of
#     mid-run failures. We always re-login here — it's idempotent and free
#     (~500ms). If it fails, that's almost always missing AcrPush role.
preflight_acr_login() {
  if [[ "$DRY_RUN" == "1" ]]; then
    info "(dry-run: would run az acr login --name $ACR_NAME)"
    return 0
  fi
  az acr login --name "$ACR_NAME" >/dev/null 2>&1
}
check "ACR login fresh ($ACR_NAME)" preflight_acr_login

# 0.5 Current kubectl context. We don't switch — we explicitly pass --context
#     in every later kubectl call — but if the operator is on the wrong
#     context, side commands they run (kubectl logs, etc.) will surprise
#     them. Warn loudly.
CURRENT_CTX="$(kubectl config current-context 2>/dev/null || echo '<none>')"
info "kubectl current-context: ${C_BOLD}$CURRENT_CTX${C_OFF}"
if [[ "$CURRENT_CTX" != "$CTX_LOCAL" && "$CURRENT_CTX" != "$CTX_AKS" ]]; then
  warn "current-context is neither '$CTX_LOCAL' nor '$CTX_AKS' — ad-hoc kubectl commands may target the wrong cluster"
fi

# 0.6 Both required contexts exist (only check AKS if we're going there).
check_ctx_exists() { kubectl config get-contexts -o name | grep -qx "$1"; }

check "context '$CTX_LOCAL' exists" check_ctx_exists "$CTX_LOCAL"
if [[ "$NO_AKS" == "0" ]]; then
  check "context '$CTX_AKS' exists"   check_ctx_exists "$CTX_AKS"
fi

# 0.7 Each context can actually reach its apiserver (quick & cheap).
check_ctx_reachable() { kubectl --context="$1" --request-timeout=5s get --raw='/version' >/dev/null 2>&1; }
check "context '$CTX_LOCAL' apiserver reachable" check_ctx_reachable "$CTX_LOCAL"
if [[ "$NO_AKS" == "0" ]]; then
  check "context '$CTX_AKS' apiserver reachable"   check_ctx_reachable "$CTX_AKS"
fi

# Summary line. Mars asked for "5/5 ✅" style output.
echo ""
if [[ ${#PREFLIGHT_FAIL_REASONS[@]} -eq 0 ]]; then
  ok "Pre-flight: $PREFLIGHT_PASS/$PREFLIGHT_TOTAL passed"
else
  err "Pre-flight: $PREFLIGHT_PASS/$PREFLIGHT_TOTAL passed (${#PREFLIGHT_FAIL_REASONS[@]} fatal)"
  echo ""
  echo "  Recovery hints:" >&2
  for r in "${PREFLIGHT_FAIL_REASONS[@]}"; do
    case "$r" in
      *"az logged in"*)   echo "    → az login --tenant <your-tenant-id>" >&2 ;;
      *"ACR login"*)      echo "    → az acr login --name $ACR_NAME  (if it still fails: az role assignment list --assignee \$(az ad signed-in-user show --query id -o tsv) --scope \$(az acr show -n $ACR_NAME --query id -o tsv))" >&2 ;;
      *"context "*"exists"*)  echo "    → az aks get-credentials --name <cluster> --resource-group <rg> --overwrite-existing" >&2 ;;
      *"apiserver reachable"*) echo "    → check VPN / Tailscale; for docker-desktop ensure Kubernetes is enabled in Settings → Kubernetes" >&2 ;;
      *"buildx"*)         echo "    → docker buildx install  (or update Docker Desktop to 4.x+)" >&2 ;;
      *)                  echo "    → $r" >&2 ;;
    esac
  done
  exit 1
fi

# ════════════════════════════════════════════════════════════════════════════
# PHASE 1 — Resolve build scope (--auto detection)
# ════════════════════════════════════════════════════════════════════════════
# Goal: don't waste ~40s building frontend if Mars only touched a Go file.
# --auto reads `git status --porcelain` for each subdir; flags override.
# If --auto + nothing changed, we default to "build both" (safer than
# "build nothing" — which would print "everything succeeded" with no work
# done, the exact false-complete pattern we're fighting).
banner "Phase 1 — Build scope"

if [[ "$AUTO_DETECT" == "1" ]]; then
  # git status returns "" when clean. We separately probe both subdirs.
  BACKEND_DIRTY=0
  FRONTEND_DIRTY=0
  if [[ -d "$BACKEND_DIR/.git" ]] || git -C "$BACKEND_DIR" rev-parse 2>/dev/null; then
    [[ -n "$(git -C "$BACKEND_DIR" status --porcelain 2>/dev/null || true)" ]] && BACKEND_DIRTY=1
  fi
  if [[ -d "$FRONTEND_DIR/.git" ]] || git -C "$FRONTEND_DIR" rev-parse 2>/dev/null; then
    [[ -n "$(git -C "$FRONTEND_DIR" status --porcelain 2>/dev/null || true)" ]] && FRONTEND_DIRTY=1
  fi

  if [[ "$BACKEND_DIRTY" == "0" && "$FRONTEND_DIRTY" == "0" ]]; then
    warn "--auto found no uncommitted changes in either subdir; defaulting to BOTH (safer than no-op)"
  else
    DO_BACKEND="$BACKEND_DIRTY"
    DO_FRONTEND="$FRONTEND_DIRTY"
    info "--auto detected: backend=$BACKEND_DIRTY frontend=$FRONTEND_DIRTY"
  fi
fi

if [[ "$DO_BACKEND" == "0" && "$DO_FRONTEND" == "0" ]]; then
  fatal 1 "Nothing to build (both --backend-only and --frontend-only were suppressed?)" \
    "Re-run without the suppressing flag, or use --auto."
fi

[[ "$DO_BACKEND"  == "1" ]] && ok "Will build BACKEND"  || info "Skipping backend"
[[ "$DO_FRONTEND" == "1" ]] && ok "Will build FRONTEND" || info "Skipping frontend"

# ════════════════════════════════════════════════════════════════════════════
# PHASE 2 — Generate VERSION tag
# ════════════════════════════════════════════════════════════════════════════
# Format: 0.9.1.10-alpha-dev-MMDDHHMM
# Why include MMDDHHMM: dev tags get reused dozens of times in a session;
# embedding the wall-clock minute makes `kubectl get deploy -o jsonpath`
# trivially diff-able ("oh, my deploy is still on 05301148 — last push was
# 1149, deploy didn't take"). The minute granularity is enough — back-to-back
# dev-deploys are >60s apart by definition (the verification phase alone is
# ~10s).
banner "Phase 2 — Generate version tag"

BASE_VERSION="${SUPKUBE_DEV_BASE_VERSION:-0.9.1.10-alpha}"
TIMESTAMP="$(date +%m%d%H%M)"
VERSION="${BASE_VERSION}-dev-${TIMESTAMP}"
echo "$VERSION" > "$TMP_VERSION_FILE"
ok "VERSION = ${C_BOLD}$VERSION${C_OFF}"
info "(written to $TMP_VERSION_FILE)"

# ════════════════════════════════════════════════════════════════════════════
# PHASE 3 — Build (per-arch, in parallel where safe)
# ════════════════════════════════════════════════════════════════════════════
# Strategy:
#   - arm64 → `--load` into local docker daemon → docker-desktop pulls
#     from the daemon (no registry round-trip)
#   - amd64 → `--push` to ACR → AKS pulls from ACR
# We do NOT use a multi-arch manifest list here (publish-release.sh does
# that for stable releases). Manifest lists need `--push` for BOTH arches,
# which costs an extra ~15s arm64 push that we'd never pull. For dev
# iteration, two separate tags with -arm64 / -amd64 suffix is faster.
#
# Backend and frontend builds run in PARALLEL when both selected — they
# share zero state (different Dockerfiles, different contexts) and the GIL
# is on network not CPU. Saves ~30-40% wall time on "build both" runs.
banner "Phase 3 — Build & push"

# Make sure the named builder exists. Copied from publish-release.sh so we
# share the cache layer. Idempotent.
if [[ "$DRY_RUN" == "0" ]]; then
  if ! docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
    info "Creating buildx builder '$BUILDX_BUILDER'…"
    docker buildx create --name "$BUILDX_BUILDER" --use --bootstrap >/dev/null
  else
    docker buildx use "$BUILDX_BUILDER" >/dev/null
  fi
fi

# build_one <component> <dockerfile> <context> <image_name>
# Builds BOTH arches for one component and reports timing. Each arch is a
# separate buildx invocation so a failure isolates cleanly (manifest-list
# failures hide which arch broke; two builds give two clear logs).
build_one() {
  local component="$1" dockerfile="$2" context="$3" image_name="$4"
  local tag_arm="${ACR_LOGIN_SERVER}/${image_name}:${VERSION}-arm64"
  local tag_amd="${ACR_LOGIN_SERVER}/${image_name}:${VERSION}-amd64"

  local t_start=$SECONDS

  # arm64 → local daemon (--load). docker-desktop's k8s pulls from the local
  # daemon when ImagePullPolicy=IfNotPresent + the tag matches a local image.
  info "[$component] building linux/arm64 → local daemon (tag $tag_arm)"
  run docker buildx build \
    --builder "$BUILDX_BUILDER" \
    --platform linux/arm64 \
    --load \
    -f "$dockerfile" \
    -t "$tag_arm" \
    --build-arg "SUPKUBE_VERSION=$VERSION" \
    "$context" \
    || fatal 2 "[$component] arm64 build failed" \
       "Inspect the buildx output above. Common causes:" \
       "  - go.sum out of date → cd $context && go mod tidy" \
       "  - npm package-lock drift → cd $context && npm install" \
       "  - disk full → docker builder prune -af --filter until=72h"

  # amd64 → push to ACR. AKS will pull from there on the next pod start.
  info "[$component] building linux/amd64 → ACR push (tag $tag_amd)"
  run docker buildx build \
    --builder "$BUILDX_BUILDER" \
    --platform linux/amd64 \
    --push \
    -f "$dockerfile" \
    -t "$tag_amd" \
    --build-arg "SUPKUBE_VERSION=$VERSION" \
    "$context" \
    || fatal 2 "[$component] amd64 build/push failed" \
       "Common causes:" \
       "  - ACR token expired mid-build (it's 1h) → az acr login --name $ACR_NAME && re-run" \
       "  - network blip → just re-run" \
       "  - missing AcrPush role → az role assignment list --assignee \$(az ad signed-in-user show --query id -o tsv) --scope \$(az acr show -n $ACR_NAME --query id -o tsv)"

  # Image size — quick eyeball check ("did the binary grow 100MB? maybe the
  # vendor dir leaked into the layer"). docker inspect is local only, so we
  # skip in dry-run.
  if [[ "$DRY_RUN" == "0" ]]; then
    local size
    size=$(docker image inspect "$tag_arm" --format '{{.Size}}' 2>/dev/null || echo 0)
    local size_mb=$((size / 1024 / 1024))
    info "[$component] arm64 image size: ${size_mb} MB"
  fi

  local t_end=$SECONDS
  ok "[$component] built in $((t_end - t_start))s"
}

# Kick off builds. We run them in the background ("&") when both selected,
# then `wait` for completion. Output interleaves, but each line is prefixed
# with [backend]/[frontend] so it's parseable. If one fails, `set -e` plus
# the `wait` exit code propagates the failure.
PIDS=()
if [[ "$DO_BACKEND" == "1" ]]; then
  (build_one "backend" "docker/Dockerfile.backend" "$BACKEND_DIR" "$BACKEND_IMAGE_NAME") &
  PIDS+=("$!")
fi
if [[ "$DO_FRONTEND" == "1" ]]; then
  (build_one "frontend" "docker/Dockerfile.frontend" "$FRONTEND_DIR" "$FRONTEND_IMAGE_NAME") &
  PIDS+=("$!")
fi

BUILD_FAILED=0
for pid in "${PIDS[@]}"; do
  wait "$pid" || BUILD_FAILED=1
done
if [[ "$BUILD_FAILED" == "1" ]]; then
  fatal 2 "One or more builds failed (see logs above)" \
    "Each component's build is independent — re-run with --backend-only or --frontend-only to isolate."
fi

ok "All builds complete"

# ════════════════════════════════════════════════════════════════════════════
# PHASE 4 — Deploy to BOTH clusters (parallel set-image + rollout)
# ════════════════════════════════════════════════════════════════════════════
# CRITICAL DETAIL — container name vs deployment name
# ───────────────────────────────────────────────────
# `kubectl set image deploy/X CONTAINER=IMAGE` fails SILENTLY (it prints
# "deployment.apps/X image updated" and exits 0) if CONTAINER doesn't match
# any container in the pod spec. The old image keeps running and the next
# rollout-status returns immediately. We've been bitten by exactly this:
# someone wrote `kubectl set image deploy/supkube-backend supkube-backend=...`
# and the deployment never rolled. The container name in our charts is just
# `backend` (and `frontend`).
#
# Defensive: probe the live deployment for the actual first-container name
# instead of trusting a hardcoded constant. Belt-and-braces in case Helm
# values change.
banner "Phase 4 — Deploy"

resolve_container_name() {
  local ctx="$1" deploy="$2" fallback="$3"
  local got
  got=$(kubectl --context="$ctx" -n "$NS_SUPKUBE" get deploy "$deploy" \
        -o jsonpath='{.spec.template.spec.containers[0].name}' 2>/dev/null || echo "")
  if [[ -z "$got" ]]; then
    echo "$fallback"
  else
    echo "$got"
  fi
}

# deploy_one <ctx> <arch_suffix> <component> <deploy_name> <fallback_container>
# Updates the image on a single deployment and waits for rollout to converge.
# Rollout timeout is intentionally tight (3 min) — if it takes longer the
# pod is probably ImagePullBackOff or CrashLoopBackOff, and we want a
# fast, loud failure instead of a 10-minute "still waiting".
deploy_one() {
  local ctx="$1" arch="$2" component="$3" deploy="$4" fallback="$5" image_name="$6"
  local image="${ACR_LOGIN_SERVER}/${image_name}:${VERSION}-${arch}"

  # Skip if the deployment doesn't exist on this cluster (might happen on a
  # fresh docker-desktop that's never had a helm install). Better to skip
  # with a warn than to fail-fast on an environmental difference.
  if [[ "$DRY_RUN" == "0" ]] && ! kubectl --context="$ctx" -n "$NS_SUPKUBE" get deploy "$deploy" >/dev/null 2>&1; then
    warn "[$ctx] deployment $deploy not found in ns/$NS_SUPKUBE — skipping (helm install missing?)"
    return 0
  fi

  local container
  container=$(resolve_container_name "$ctx" "$deploy" "$fallback")
  info "[$ctx] $deploy: setting container '$container' → $image"

  run kubectl --context="$ctx" -n "$NS_SUPKUBE" set image \
    "deploy/$deploy" "${container}=${image}" \
    || fatal 3 "[$ctx] set image failed for $deploy" \
       "Run: kubectl --context=$ctx -n $NS_SUPKUBE describe deploy $deploy"

  info "[$ctx] waiting for $deploy rollout (timeout 3m)…"
  run kubectl --context="$ctx" -n "$NS_SUPKUBE" rollout status \
    "deploy/$deploy" --timeout=3m \
    || fatal 3 "[$ctx] rollout did not converge for $deploy within 3m" \
       "Likely causes:" \
       "  - ImagePullBackOff: kubectl --context=$ctx -n $NS_SUPKUBE describe pod -l app=$deploy | grep -A3 Events" \
       "  - CrashLoopBackOff: kubectl --context=$ctx -n $NS_SUPKUBE logs -l app=$deploy --tail=50 --previous" \
       "  - AKS pulling from ACR? Confirm AcrPull role on kubelet identity"

  ok "[$ctx] $deploy rolled to $image"
}

# Build a list of (ctx, arch) pairs to deploy to. arm64 → local, amd64 → AKS.
DEPLOY_TARGETS=("$CTX_LOCAL:arm64")
if [[ "$NO_AKS" == "0" ]]; then
  DEPLOY_TARGETS+=("$CTX_AKS:amd64")
fi

# Run deploys in parallel across (cluster × component). Same fail-fast
# behaviour as Phase 3: failures propagate through `wait`.
PIDS=()
for target in "${DEPLOY_TARGETS[@]}"; do
  ctx="${target%%:*}"
  arch="${target##*:}"
  if [[ "$DO_BACKEND" == "1" ]]; then
    (deploy_one "$ctx" "$arch" "backend" "$DEPLOY_BACKEND" "$CONTAINER_BACKEND_FALLBACK" "$BACKEND_IMAGE_NAME") &
    PIDS+=("$!")
  fi
  if [[ "$DO_FRONTEND" == "1" ]]; then
    (deploy_one "$ctx" "$arch" "frontend" "$DEPLOY_FRONTEND" "$CONTAINER_FRONTEND_FALLBACK" "$FRONTEND_IMAGE_NAME") &
    PIDS+=("$!")
  fi
done

DEPLOY_FAILED=0
for pid in "${PIDS[@]}"; do
  wait "$pid" || DEPLOY_FAILED=1
done
if [[ "$DEPLOY_FAILED" == "1" ]]; then
  fatal 3 "One or more deploys failed (see logs above)" \
    "The deployments left behind are partially upgraded — Mars should decide whether to roll back: kubectl rollout undo deploy/<name> -n $NS_SUPKUBE"
fi

ok "All deploys converged"

# ════════════════════════════════════════════════════════════════════════════
# PHASE 5 — Real verification (the whole point of this script)
# ════════════════════════════════════════════════════════════════════════════
# Four independent checks per (cluster × component). Any failure = exit 4.
# These are intentionally hostile to "looks fine" outcomes:
#   1. Image tag on the deployment matches VERSION exactly. Catches the
#      silent-no-op container-name bug from Phase 4.
#   2. /api/v1/status.buildStamp matches the wall-clock minute we built
#      at. Catches the "deployment was updated but rollout actually used
#      a cached older image" case (which shouldn't happen but did once).
#   3. Frontend nginx is serving the index hash from dist/index.html.
#      Catches the "deployment rolled, but nginx still has old assets"
#      case which we hit when frontend Docker layers cached too aggressively.
#   4. ServiceAccount can list pods in supkube AND velero namespaces.
#      Issue #79: a Helm upgrade silently dropped a ClusterRoleBinding
#      and the UI started returning empty pod lists with no error in any
#      log. `kubectl auth can-i` exits 1 if the answer is "no", giving us
#      a hard, machine-readable signal.
banner "Phase 5 — Verify (the 4 hard checks)"

if [[ "$SKIP_VERIFY" == "1" ]]; then
  warn "--skip-verify was passed; skipping ALL verification. This is the false-complete trap. You have been warned."
else

# Results table; populated per (cluster × component). Printed at end.
declare -a REPORT_ROWS=()

# verify_image_tag <ctx> <deploy> <expected_arch>
# Reads the running deployment's first-container image and checks it ends
# with VERSION-<arch>. Equality, not contains — we want to fail if some
# init container's image was tagged instead of the main container.
verify_image_tag() {
  local ctx="$1" deploy="$2" arch="$3"
  if [[ "$DRY_RUN" == "1" ]]; then
    info "(dry-run: would verify image tag on $ctx/$deploy)"
    return 0
  fi
  local got expected="${ACR_LOGIN_SERVER}/${4}:${VERSION}-${arch}"
  got=$(kubectl --context="$ctx" -n "$NS_SUPKUBE" get deploy "$deploy" \
        -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "<missing>")
  if [[ "$got" == "$expected" ]]; then
    ok "[$ctx/$deploy] image tag matches: $got"
    return 0
  else
    err "[$ctx/$deploy] image tag MISMATCH"
    echo "      expected: $expected" >&2
    echo "      got:      $got" >&2
    return 1
  fi
}

# verify_buildstamp <ctx> <deploy>
# `exec wget -qO- localhost:8080/api/v1/status | jq .buildStamp`. The
# buildStamp format is YYMMDD-HHMM, set at compile time via -ldflags -X.
# We don't try to match exact minute (compile takes 20-40s; the stamp can
# be 1-2 minutes BEFORE VERSION's timestamp). We just check it's "today",
# which is sufficient to catch a serving-stale-binary regression.
# Backend only — frontend has no status endpoint.
verify_buildstamp() {
  local ctx="$1" deploy="$2"
  if [[ "$DRY_RUN" == "1" ]]; then
    info "(dry-run: would exec $deploy and check $STATUS_PATH.buildStamp)"
    return 0
  fi
  local got today
  today=$(date -u +%y%m%d)
  # Some backend images don't have wget; try curl as fallback. Both are
  # in the runtime alpine base.
  got=$(kubectl --context="$ctx" -n "$NS_SUPKUBE" exec "deploy/$deploy" -- \
        sh -c "wget -qO- http://localhost:8080${STATUS_PATH} 2>/dev/null || curl -sf http://localhost:8080${STATUS_PATH}" \
        2>/dev/null | jq -r '.buildStamp // empty' 2>/dev/null || echo "")
  if [[ -z "$got" ]]; then
    err "[$ctx/$deploy] could not read $STATUS_PATH.buildStamp (empty response or no jq path)"
    return 1
  fi
  if [[ "$got" == "${today}"* ]]; then
    ok "[$ctx/$deploy] buildStamp fresh: $got (today=$today)"
    return 0
  else
    err "[$ctx/$deploy] buildStamp NOT today: got '$got', expected '${today}-*'"
    return 1
  fi
}

# verify_frontend_asset <ctx> <deploy>
# Lists /usr/share/nginx/html/assets/ inside the pod and confirms there's
# an index-<hash>.js whose hash also exists in the local dist/. The local
# build is the source of truth; if the running pod's hash doesn't appear
# in dist/, we're serving an older bundle than what we just built.
#
# Skipped if local dist/ doesn't exist (e.g. backend-only run, or never
# ran `npm run build` locally — which is fine, the docker build did it
# inside the image).
verify_frontend_asset() {
  local ctx="$1" deploy="$2"
  if [[ "$DRY_RUN" == "1" ]]; then
    info "(dry-run: would exec $deploy and inspect /usr/share/nginx/html/assets/)"
    return 0
  fi
  local pod_assets
  pod_assets=$(kubectl --context="$ctx" -n "$NS_SUPKUBE" exec "deploy/$deploy" -- \
               sh -c 'ls /usr/share/nginx/html/assets/ 2>/dev/null | grep -E "^index-.*\.(js|css)$" | head -5' \
               2>/dev/null || echo "")
  if [[ -z "$pod_assets" ]]; then
    err "[$ctx/$deploy] no index-*.{js,css} found in /usr/share/nginx/html/assets/"
    return 1
  fi
  # We're not comparing against dist/ (often absent on a clean dev mac) —
  # the existence of a fresh hashed bundle plus a successful rollout is
  # enough signal. The real bug we're guarding against (stale layer cache)
  # would surface as `pod_assets` being empty or having the OLD hash.
  ok "[$ctx/$deploy] nginx assets present: $(echo "$pod_assets" | tr '\n' ' ')"
  return 0
}

# verify_sa_rbac <ctx> <ns> — issue #79 protection
# `kubectl auth can-i` returns "yes" / "no" on stdout and exit-0 / exit-1
# matching. Exits 1 even when stdout says "no", which is what we want for
# automation. The --as=system:serviceaccount:... flag impersonates the SA
# without needing to actually log in as it.
verify_sa_rbac() {
  local ctx="$1" ns="$2"
  if [[ "$DRY_RUN" == "1" ]]; then
    info "(dry-run: would check SA can list pods in $ns)"
    return 0
  fi
  # v0.9.x fix (2026-05-31): 不再 hardcode SA name="$DEPLOY_BACKEND" — 之前假设
  # SA 跟 deployment 同名 (supkube-backend), 但实际 helm chart 给的是 release
  # name "supkube" 一字。Probe 真 spec.serviceAccountName 拿到真名再 check
  # 权限——同 Phase 4 deploy 时 probe container name 的做法。
  local sa_name
  sa_name=$(kubectl --context="$ctx" -n "$NS_SUPKUBE" get deploy "$DEPLOY_BACKEND" \
              -o jsonpath='{.spec.template.spec.serviceAccountName}' 2>/dev/null)
  if [[ -z "$sa_name" ]]; then
    # SA 字段空 = pod 用 ns 的 default SA
    sa_name="default"
    warn "[$ctx] deploy/$DEPLOY_BACKEND 没设 serviceAccountName, 用 ns default — 这本身就是配置 smell"
  fi
  local sa="system:serviceaccount:${NS_SUPKUBE}:${sa_name}"
  local got
  got=$(kubectl --context="$ctx" auth can-i list pods -n "$ns" --as="$sa" 2>/dev/null || echo "no")
  if [[ "$got" == "yes" ]]; then
    ok "[$ctx] SA $sa can list pods in ns/$ns"
    return 0
  else
    err "[$ctx] SA $sa CANNOT list pods in ns/$ns (auth can-i=$got)"
    echo "      → this is the #79 silent-RBAC-drop signature" >&2
    echo "      → kubectl --context=$ctx get clusterrolebinding | grep $sa_name" >&2
    return 1
  fi
}

VERIFY_FAILED=0

# Run verification for each (ctx, arch) pair × component selection.
for target in "${DEPLOY_TARGETS[@]}"; do
  ctx="${target%%:*}"
  arch="${target##*:}"
  echo ""
  echo -e "  ${C_BOLD}── cluster: $ctx (arch: $arch) ──${C_OFF}"

  if [[ "$DO_BACKEND" == "1" ]]; then
    verify_image_tag "$ctx" "$DEPLOY_BACKEND" "$arch" "$BACKEND_IMAGE_NAME" \
      && r1=OK || { r1=FAIL; VERIFY_FAILED=1; }
    verify_buildstamp "$ctx" "$DEPLOY_BACKEND" \
      && r2=OK || { r2=FAIL; VERIFY_FAILED=1; }
    REPORT_ROWS+=("$ctx|backend|${VERSION}-${arch}|img:$r1 stamp:$r2")
  fi

  if [[ "$DO_FRONTEND" == "1" ]]; then
    verify_image_tag "$ctx" "$DEPLOY_FRONTEND" "$arch" "$FRONTEND_IMAGE_NAME" \
      && r3=OK || { r3=FAIL; VERIFY_FAILED=1; }
    verify_frontend_asset "$ctx" "$DEPLOY_FRONTEND" \
      && r4=OK || { r4=FAIL; VERIFY_FAILED=1; }
    REPORT_ROWS+=("$ctx|frontend|${VERSION}-${arch}|img:$r3 asset:$r4")
  fi

  # RBAC check is per-cluster, not per-component — only meaningful when
  # backend was deployed (it owns the SA). Run unconditionally though:
  # the SA exists even if we only rebuilt frontend, and a silent RBAC
  # drop earlier in the week would still be worth surfacing here.
  verify_sa_rbac "$ctx" "$NS_SUPKUBE" \
    && r5=OK || { r5=FAIL; VERIFY_FAILED=1; }
  verify_sa_rbac "$ctx" "$NS_VELERO" \
    && r6=OK || { r6=FAIL; VERIFY_FAILED=1; }
  REPORT_ROWS+=("$ctx|rbac|n/a|supkube:$r5 velero:$r6")
done

if [[ "$VERIFY_FAILED" == "1" ]]; then
  echo ""
  fatal 4 "One or more verifications failed — DO NOT trust this deploy" \
    "Re-read the per-check output above; the most common pattern is image-tag-OK + buildStamp-stale, which means the pod is running an older binary than its declared image tag (cached layer or wrong arch)."
fi

fi   # end SKIP_VERIFY guard

# ════════════════════════════════════════════════════════════════════════════
# PHASE 6 — Final report
# ════════════════════════════════════════════════════════════════════════════
banner "Phase 6 — Report"

if [[ "$SKIP_VERIFY" == "0" ]]; then
  printf "  %-22s %-10s %-46s %s\n" "CLUSTER" "COMPONENT" "TAG" "CHECKS"
  printf "  %-22s %-10s %-46s %s\n" "──────────────────────" "──────────" "──────────────────────────────────────────────" "──────────────────────"
  for row in "${REPORT_ROWS[@]}"; do
    IFS='|' read -r ctx comp tag checks <<<"$row"
    printf "  %-22s %-10s %-46s %s\n" "$ctx" "$comp" "$tag" "$checks"
  done
  echo ""
fi

ok "${C_BOLD}Dev deploy SUCCESS${C_OFF} — version ${C_BOLD}$VERSION${C_OFF}"
echo ""
echo "  Browser checks:"
echo -e "    ${C_DIM}docker-desktop:${C_OFF} https://localhost:30888"
if [[ "$NO_AKS" == "0" ]]; then
  # We don't try to compute the actual AKS ingress URL — depends on
  # cluster networking. Just remind Mars where to look.
  echo -e "    ${C_DIM}AKS ($CTX_AKS):${C_OFF}      kubectl --context=$CTX_AKS -n $NS_SUPKUBE get ingress  (then visit the host shown)"
fi
echo ""
echo "  If the UI still looks stale: hard-reload (Cmd+Shift+R); index.html is no-cache but Safari sometimes ignores that."
echo ""

exit 0
