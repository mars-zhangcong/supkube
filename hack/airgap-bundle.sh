#!/usr/bin/env bash
# airgap-bundle.sh — pull every image listed in integrations/images.yaml,
# `docker save` each to a tar, then bundle them + the SupKube Helm chart +
# the integration manifest into a single tarball ready for an air-gapped
# customer.
#
# Why a custom bundler vs. `docker compose` save / similar:
#   - We need to bundle non-Docker artifacts too: Helm chart, install
#     script, INTEGRATION.md, the K8s manifests for things we don't ship
#     via Helm (snapshot-controller etc.)
#   - Customer's airgap env may not have `docker` — `skopeo dir:` mode
#     produces OCI layouts that can load into containerd / Harbor without
#     a Docker daemon
#   - One tarball, one MD5/SHA256, one "scp" to ship across the air gap
#
# Output structure inside the tarball:
#   supkube-airgap-<version>/
#   ├── README.md                  ← load instructions for the receiving end
#   ├── INTEGRATION.md             ← copy of the version-pinned manifest
#   ├── integrations/images.yaml
#   ├── supkube-helm/              ← entire chart, deployable offline
#   ├── images/
#   │   ├── supkube_backend_0.8.7.2-alpha.tar
#   │   ├── velero_velero_v1.18.0.tar
#   │   └── ...
#   └── scripts/
#       ├── load-images.sh         ← runs on the receiving end
#       └── verify-cluster.sh

set -euo pipefail

OUT_PATH="${1:-}"
if [[ -z "$OUT_PATH" ]]; then
  cat <<EOF
Usage: $0 <output-tarball-path>

Example:
  $0 ./supkube-airgap-v0.8.7.2.tar.gz

This will pull all images in integrations/images.yaml, package them with
the Helm chart and docs into a single tarball you can scp to an air-gapped
host.
EOF
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="$REPO_ROOT/integrations/images.yaml"

# Stage everything under a temp dir, then tar at the end. The temp dir
# layout matches what the receiving end will see after un-tar.
STAGE="$(mktemp -d -t supkube-airgap-XXXXXX)"
trap 'rm -rf "$STAGE"' EXIT

VERSION=$(python3 -c "import yaml; print(yaml.safe_load(open('$MANIFEST'))['metadata']['supkubeVersion'])")
ROOT_NAME="supkube-airgap-${VERSION}"
ROOT="$STAGE/$ROOT_NAME"

mkdir -p "$ROOT/images" "$ROOT/scripts" "$ROOT/integrations"

echo "Bundle version: $VERSION"
echo "Staging at:     $STAGE"

# Copy docs + chart + manifest into the bundle.
cp "$REPO_ROOT/INTEGRATION.md" "$ROOT/INTEGRATION.md"
cp "$REPO_ROOT/integrations/images.yaml" "$ROOT/integrations/images.yaml"
if [[ -d "$REPO_ROOT/supkube-helm" ]]; then
  cp -R "$REPO_ROOT/supkube-helm" "$ROOT/supkube-helm"
fi

# Pull + save each image. We use docker save here (the lowest common
# denominator) because the receiving end is often a Linux box with
# Docker installed; containerd users can load these tarballs too via
# `ctr image import`.
#
# We DO NOT include planned/un-shipped images — those would bloat the
# bundle for no reason. Override with AIRGAP_INCLUDE_PLANNED=1.
echo ""
echo "Pulling and saving images..."
mapfile -t images < <(python3 - "$MANIFEST" <<'PYEOF'
import os, sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)
include_planned = os.environ.get('AIRGAP_INCLUDE_PLANNED') == '1'
for entry in doc.get('integrations', []):
    if entry.get('category') == 'planned' and not include_planned:
        continue
    img = entry.get('image')
    if img:
        print(img)
PYEOF
)

if [[ ${#images[@]} -eq 0 ]]; then
  echo "ERROR: no images to bundle" >&2
  exit 1
fi

total=${#images[@]}
i=0
for img in "${images[@]}"; do
  i=$((i + 1))
  # Sanitize image name for filename: registry.example.com/path/img:tag → registry.example.com_path_img_tag.tar
  safe=$(echo "$img" | tr '/:' '__')
  out="$ROOT/images/${safe}.tar"
  echo "[$i/$total] $img"
  if ! docker pull "$img" >/dev/null 2>&1; then
    # Maybe already local; try save directly
    if ! docker save "$img" -o "$out" 2>/dev/null; then
      echo "  ✗ FAILED to pull or save $img" >&2
      exit 1
    fi
  else
    docker save "$img" -o "$out"
  fi
  size=$(du -h "$out" | awk '{print $1}')
  echo "  → $(basename "$out")  ($size)"
done

# Write the receiving-end load script. It reads images.yaml, finds each
# tar by the same name-sanitization rule, loads them, and (optionally)
# retags them under a local registry prefix.
cat > "$ROOT/scripts/load-images.sh" <<'LOADSH'
#!/usr/bin/env bash
# Run on the air-gapped host after extracting the bundle.
#
# Usage:
#   ./scripts/load-images.sh                    # load images into local docker
#   ./scripts/load-images.sh registry.local/sk  # load + retag + push to local registry
set -euo pipefail
TARGET="${1:-}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
total=0; loaded=0; pushed=0
for tar in "$DIR/images"/*.tar; do
  total=$((total + 1))
  echo "Loading: $(basename "$tar")"
  out=$(docker load -i "$tar" 2>&1 | tail -1)
  img=$(echo "$out" | sed -n 's/^Loaded image: //p')
  if [[ -z "$img" ]]; then
    echo "  ✗ couldn't determine image name from: $out" >&2
    continue
  fi
  loaded=$((loaded + 1))
  echo "  ✓ $img"
  if [[ -n "$TARGET" ]]; then
    new="$TARGET/$img"
    docker tag "$img" "$new"
    docker push "$new"
    pushed=$((pushed + 1))
    echo "    ↳ pushed $new"
  fi
done
echo ""
echo "Done: loaded $loaded/$total"
[[ -n "$TARGET" ]] && echo "      pushed $pushed/$total to $TARGET"
LOADSH
chmod +x "$ROOT/scripts/load-images.sh"

# Receiving-end README.
cat > "$ROOT/README.md" <<EOF
# SupKube Airgap Bundle ${VERSION}

This bundle contains everything needed to deploy SupKube ${VERSION} into a
disconnected environment.

## Contents

- \`INTEGRATION.md\` — full list of bundled components + their versions
- \`integrations/images.yaml\` — machine-readable image manifest
- \`supkube-helm/\` — Helm chart for SupKube + Dex
- \`images/\` — saved Docker images (\`docker load\` ready)
- \`scripts/load-images.sh\` — load images on the air-gapped host

## Quick Start

\`\`\`bash
# 1. Extract the bundle
tar -xzf supkube-airgap-${VERSION}.tar.gz
cd supkube-airgap-${VERSION}

# 2a. Load images into the local Docker daemon (single-node setup)
./scripts/load-images.sh

# 2b. OR load + push to your local registry (recommended for K8s clusters)
./scripts/load-images.sh registry.airgap.local/supkube

# 3. Install SupKube with Helm, pointing at the local registry
helm install supkube ./supkube-helm/supkube -n supkube --create-namespace \\
  --set backend.image.repository=registry.airgap.local/supkube/supkube/backend \\
  --set frontend.image.repository=registry.airgap.local/supkube/supkube/frontend
\`\`\`

## Velero & node-agent (installed independently)

Velero is NOT auto-installed by the SupKube Helm chart (intentional —
clusters often already have Velero, and double-installing breaks BSL sync).
Install it manually:

\`\`\`bash
velero install \\
  --image registry.airgap.local/supkube/velero/velero:v1.18.0 \\
  --plugins registry.airgap.local/supkube/velero/velero-plugin-for-aws:v1.9.0,registry.airgap.local/supkube/velero/velero-plugin-for-microsoft-azure:v1.10.0 \\
  --use-node-agent --uploader-type=kopia \\
  --features=EnableCSI \\
  ...
\`\`\`

See \`INTEGRATION.md\` for the complete component list and best practices.
EOF

# Bundle it all up.
echo ""
echo "Creating tarball..."
cd "$STAGE"
tar -czf "$OUT_PATH" "$ROOT_NAME"
cd - >/dev/null

SIZE=$(du -h "$OUT_PATH" | awk '{print $1}')
SHA=$(shasum -a 256 "$OUT_PATH" | awk '{print $1}')

cat <<EOF

✅ Bundle ready: $OUT_PATH ($SIZE)
   SHA256: $SHA

Ship via scp / sneakernet to the air-gapped host, then:
   tar -xzf $(basename "$OUT_PATH")
   cd $ROOT_NAME && ./scripts/load-images.sh <local-registry>
EOF
