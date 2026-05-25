#!/usr/bin/env bash
# mirror-images.sh — copy every image listed in integrations/images.yaml to
# a target registry. Used for:
#
#   1. Customer private registry (Harbor / Artifactory / Nexus) — they want
#      every dependency under their own URL for IP / firewall / scan reasons
#   2. Air-gapped staging — push to a local registry inside the disconnected
#      network before deploying SupKube
#
# Tool choice: we use `crane` (https://github.com/google/go-containerregistry)
# rather than `docker pull && docker push` because:
#   - No local Docker daemon required (works on CI runners + minimal hosts)
#   - Streams blobs directly registry-to-registry; no local disk hop
#   - Preserves multi-arch manifest lists by default
#
# Fallback: if `crane` isn't installed, we degrade to `skopeo copy`. If
# neither is available we print a clear "please install one" message.
#
# Usage:
#   hack/mirror-images.sh <target-registry-prefix>
#
# Example:
#   hack/mirror-images.sh harbor.example.com/supkube
#
# After running, every image gets a new tag under <prefix>. For instance:
#   velero/velero:v1.18.0  →  harbor.example.com/supkube/velero/velero:v1.18.0
#
# We DON'T rename the repo path — that would force users to remember the
# mapping for every image. Just prepend the registry prefix.

set -euo pipefail

TARGET="${1:-}"
if [[ -z "$TARGET" ]]; then
  cat <<EOF
Usage: $0 <target-registry-prefix>

Examples:
  $0 harbor.example.com/supkube
  $0 my-registry.internal:5000/dr-platform
  $0 localhost:5000  (local test)

The script reads integrations/images.yaml and copies every listed image
to <target>/<original-image-path>:<tag>.
EOF
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="$REPO_ROOT/integrations/images.yaml"

if [[ ! -f "$MANIFEST" ]]; then
  echo "ERROR: $MANIFEST not found" >&2
  exit 1
fi

# Choose a copy tool. We try crane first (smallest, fastest), then skopeo
# (more widely installed). Both are registry-to-registry with no Docker
# daemon. Last resort: `docker pull && tag && push` (heavy, requires daemon).
COPY_TOOL=""
if command -v crane >/dev/null 2>&1; then
  COPY_TOOL=crane
elif command -v skopeo >/dev/null 2>&1; then
  COPY_TOOL=skopeo
elif command -v docker >/dev/null 2>&1; then
  COPY_TOOL=docker
else
  echo "ERROR: need one of: crane, skopeo, docker" >&2
  echo "  Recommended:  go install github.com/google/go-containerregistry/cmd/crane@latest" >&2
  echo "  Or:           brew install crane    /    yum install skopeo" >&2
  exit 1
fi
echo "Using copy tool: $COPY_TOOL"

# Parse images.yaml. We don't want a yq dependency, so use Python which
# is on every modern Linux/Mac. The grep below extracts only the `image:`
# lines under `integrations:` — comments and other keys are ignored.
images=$(python3 - "$MANIFEST" <<'PYEOF'
import sys, yaml
with open(sys.argv[1]) as f:
    doc = yaml.safe_load(f)
for entry in doc.get('integrations', []):
    # Skip planned (not-yet-shipped) images by default. Override with
    # MIRROR_INCLUDE_PLANNED=1 to mirror everything (useful for staging
    # a future release).
    if entry.get('category') == 'planned' and __import__('os').environ.get('MIRROR_INCLUDE_PLANNED') != '1':
        continue
    img = entry.get('image')
    if img:
        print(img)
PYEOF
)

if [[ -z "$images" ]]; then
  echo "No images to mirror." >&2
  exit 1
fi

echo ""
echo "Will mirror to: $TARGET"
echo "$images" | sed 's/^/  /'
echo ""

# Copy loop. Each image: source = original, destination = <TARGET>/<image>.
# We keep the full original path (registry domain + repo) under the target
# so the mapping is "literal prefix replacement" — easy to read in Helm
# values and easy to diff.
total=0
failed=0
while IFS= read -r src; do
  [[ -z "$src" ]] && continue
  total=$((total + 1))
  dst="${TARGET}/${src}"

  echo "[$total] $src"
  echo "      → $dst"
  case "$COPY_TOOL" in
    crane)
      if ! crane copy --allow-nondistributable-artifacts "$src" "$dst"; then
        echo "      ✗ FAILED" >&2
        failed=$((failed + 1))
      fi
      ;;
    skopeo)
      if ! skopeo copy --all "docker://$src" "docker://$dst"; then
        echo "      ✗ FAILED" >&2
        failed=$((failed + 1))
      fi
      ;;
    docker)
      if ! { docker pull "$src" && docker tag "$src" "$dst" && docker push "$dst"; }; then
        echo "      ✗ FAILED" >&2
        failed=$((failed + 1))
      fi
      ;;
  esac
done <<< "$images"

echo ""
if [[ $failed -eq 0 ]]; then
  echo "✅ Mirrored $total/$total images to $TARGET"
else
  echo "⚠️  $((total - failed))/$total mirrored; $failed failed — see log above" >&2
  exit 1
fi

cat <<EOF

Next step — pass the new image locations to Helm:

  helm upgrade supkube ./supkube-helm/supkube \\
    --set backend.image.repository=$TARGET/supkube/backend \\
    --set frontend.image.repository=$TARGET/supkube/frontend \\
    --set auth.dex.image.repository=$TARGET/dexidp/dex

For Velero (independent install), point its CLI at the mirrored plugin:

  velero plugin add $TARGET/velero/velero-plugin-for-microsoft-azure:v1.10.0
EOF
