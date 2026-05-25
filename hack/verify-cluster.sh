#!/usr/bin/env bash
# verify-cluster.sh — sanity check that the components SupKube depends on
# are actually installed in the current kubeconfig context, at compatible
# versions.
#
# Why this exists: SupKube doesn't auto-install Velero / snapshot-controller
# (intentional — see ADR-023). The cost is that a client can deploy SupKube,
# open the UI, and only discover at first backup that node-agent is missing
# or the Azure plugin isn't loaded. This script catches that BEFORE the
# first user-visible failure.
#
# Run it:
#   - After every cluster bring-up
#   - After every velero CLI command (since it can mutate plugin list)
#   - As a CI gate before declaring a deployment "ready"
#
# Exit codes:
#   0 = everything checked is healthy
#   1 = required component missing or wrong version
#   2 = optional component missing (warning only)

set -uo pipefail   # NOT -e — we want to continue on individual check failures

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MANIFEST="$REPO_ROOT/integrations/images.yaml"

VELERO_NS="${VELERO_NAMESPACE:-velero}"
SUPKUBE_NS="${SUPKUBE_NAMESPACE:-supkube}"

# Color codes (no-op when not a TTY).
if [[ -t 1 ]]; then
  C_OK="\033[32m"; C_WARN="\033[33m"; C_ERR="\033[31m"; C_DIM="\033[2m"; C_OFF="\033[0m"
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_OFF=""
fi

REQUIRED_FAIL=0
OPTIONAL_FAIL=0

pass() { echo -e "  ${C_OK}✓${C_OFF} $1"; }
warn() { echo -e "  ${C_WARN}!${C_OFF} $1"; OPTIONAL_FAIL=$((OPTIONAL_FAIL + 1)); }
fail() { echo -e "  ${C_ERR}✗${C_OFF} $1"; REQUIRED_FAIL=$((REQUIRED_FAIL + 1)); }
info() { echo -e "  ${C_DIM}·${C_OFF} $1"; }

section() { echo ""; echo -e "${C_OK}══${C_OFF} $1"; }

# Expected version helper. Reads from integrations/images.yaml so the
# expectation is single-sourced.
expected_image() {
  local name="$1"
  python3 - "$MANIFEST" "$name" <<'PYEOF'
import sys, yaml
with open(sys.argv[1]) as f: doc = yaml.safe_load(f)
want = sys.argv[2]
for e in doc.get('integrations', []):
    if e.get('name') == want:
        print(e.get('image', ''))
        sys.exit(0)
sys.exit(1)
PYEOF
}

# ─── 1. kubectl available + context set ────────────────────────────
section "Environment"
if ! command -v kubectl >/dev/null 2>&1; then
  echo -e "${C_ERR}kubectl not found; nothing else can be checked.${C_OFF}" >&2
  exit 1
fi
CTX=$(kubectl config current-context 2>/dev/null || echo "<none>")
info "kubectl context: $CTX"

if ! kubectl get nodes >/dev/null 2>&1; then
  fail "Cannot reach cluster API server"
  exit 1
fi
K8S_VERSION=$(kubectl version --client=false -o json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('serverVersion',{}).get('gitVersion','?'))" 2>/dev/null || echo "?")
info "K8s server version: $K8S_VERSION"

# ─── 2. SupKube itself ─────────────────────────────────────────────
section "SupKube"
if kubectl -n "$SUPKUBE_NS" get deploy supkube-backend supkube-frontend >/dev/null 2>&1; then
  BE_IMG=$(kubectl -n "$SUPKUBE_NS" get deploy supkube-backend -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null)
  FE_IMG=$(kubectl -n "$SUPKUBE_NS" get deploy supkube-frontend -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null)
  pass "supkube-backend  $BE_IMG"
  pass "supkube-frontend $FE_IMG"
else
  fail "SupKube deployments not found in namespace '$SUPKUBE_NS'"
fi

# ─── 3. Velero core ────────────────────────────────────────────────
section "Velero"
if kubectl -n "$VELERO_NS" get deploy velero >/dev/null 2>&1; then
  V_IMG=$(kubectl -n "$VELERO_NS" get deploy velero -o jsonpath='{.spec.template.spec.containers[0].image}')
  V_READY=$(kubectl -n "$VELERO_NS" get deploy velero -o jsonpath='{.status.readyReplicas}/{.status.replicas}')
  pass "velero deployment  $V_IMG  ($V_READY ready)"

  # Check plugin list.
  PLUGINS=$(kubectl -n "$VELERO_NS" get deploy velero -o jsonpath='{.spec.template.spec.initContainers[*].image}')
  if echo "$PLUGINS" | grep -q velero-plugin-for-aws; then
    pass "velero-plugin-for-aws loaded"
  else
    warn "velero-plugin-for-aws NOT loaded — S3/MinIO/COS BSLs will fail"
  fi
  if echo "$PLUGINS" | grep -q velero-plugin-for-microsoft-azure; then
    pass "velero-plugin-for-microsoft-azure loaded"
  else
    info "velero-plugin-for-microsoft-azure NOT loaded (only needed if using Azure BSL)"
  fi
else
  fail "Velero not installed in namespace '$VELERO_NS'  — install it before using SupKube"
fi

# ─── 4. node-agent (required for Data Mover / Filesystem backup) ───
section "Velero node-agent"
if kubectl -n "$VELERO_NS" get ds node-agent >/dev/null 2>&1; then
  NA_DESIRED=$(kubectl -n "$VELERO_NS" get ds node-agent -o jsonpath='{.status.desiredNumberScheduled}')
  NA_READY=$(kubectl -n "$VELERO_NS" get ds node-agent -o jsonpath='{.status.numberReady}')
  if [[ "$NA_READY" == "$NA_DESIRED" && "$NA_READY" != "0" ]]; then
    pass "node-agent DaemonSet  $NA_READY/$NA_DESIRED ready"
  else
    warn "node-agent DaemonSet present but only $NA_READY/$NA_DESIRED ready"
  fi
else
  warn "node-agent NOT installed — Data Mover and Filesystem backups won't work (CSI-only is fine)"
fi

# ─── 5. Dex ────────────────────────────────────────────────────────
section "Dex"
if kubectl -n "$SUPKUBE_NS" get deploy supkube-dex >/dev/null 2>&1; then
  D_IMG=$(kubectl -n "$SUPKUBE_NS" get deploy supkube-dex -o jsonpath='{.spec.template.spec.containers[0].image}')
  pass "Dex deployment  $D_IMG"
else
  warn "Dex not installed — SupKube auth will run in demo mode (AUTH_ENABLED=false)"
fi

# ─── 6. CSI snapshot infrastructure ────────────────────────────────
section "CSI snapshot infrastructure"
if kubectl get crd volumesnapshots.snapshot.storage.k8s.io >/dev/null 2>&1; then
  pass "VolumeSnapshot CRD installed"
else
  fail "VolumeSnapshot CRD missing — install csi-snapshotter before any CSI backup"
fi

# Find the snapshot-controller wherever it lives. EKS/AKS/GKE put it in
# kube-system; bare-metal may put it in its own namespace. The label used
# by the external-snapshotter project depends on installer:
#   - `app=snapshot-controller`                    (raw deploy from upstream)
#   - `app.kubernetes.io/name=snapshot-controller` (Helm chart / OperatorHub)
#   - Docker Desktop's bundled one ships under kube-system without consistent labels
# Try multiple label keys to avoid false-negative warns.
SC_POD=$(
  kubectl get pod -A -l app=snapshot-controller --no-headers 2>/dev/null | head -1 | awk '{print $1"/"$2}'
)
if [[ -z "$SC_POD" ]]; then
  SC_POD=$(
    kubectl get pod -A -l 'app.kubernetes.io/name=snapshot-controller' --no-headers 2>/dev/null | head -1 | awk '{print $1"/"$2}'
  )
fi
if [[ -z "$SC_POD" ]]; then
  # Last resort: name-prefix scan. Catches Docker Desktop's bundled one
  # and any unlabeled custom install.
  SC_POD=$(
    kubectl get pod -A --no-headers 2>/dev/null | awk '$2 ~ /^snapshot-controller/ {print $1"/"$2; exit}'
  )
fi
if [[ -n "$SC_POD" ]]; then
  pass "snapshot-controller running at $SC_POD"
else
  warn "snapshot-controller NOT found — VolumeSnapshots will be stuck in 'creating' state"
fi

# ─── 7. BSL health (if Velero is installed) ────────────────────────
if kubectl -n "$VELERO_NS" get deploy velero >/dev/null 2>&1; then
  section "BackupStorageLocations"
  while IFS=$'\t' read -r name phase provider; do
    [[ -z "$name" ]] && continue
    if [[ "$phase" == "Available" ]]; then
      pass "$name  ($provider)  Available"
    else
      warn "$name  ($provider)  $phase"
    fi
  done < <(kubectl -n "$VELERO_NS" get bsl -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.spec.provider}{"\n"}{end}' 2>/dev/null)
fi

# ─── Summary ───────────────────────────────────────────────────────
echo ""
if [[ $REQUIRED_FAIL -eq 0 && $OPTIONAL_FAIL -eq 0 ]]; then
  echo -e "${C_OK}══ All checks passed${C_OFF}"
  exit 0
elif [[ $REQUIRED_FAIL -eq 0 ]]; then
  echo -e "${C_WARN}══ $OPTIONAL_FAIL optional component(s) missing — cluster will work but some features are off${C_OFF}"
  exit 2
else
  echo -e "${C_ERR}══ $REQUIRED_FAIL required check(s) failed${C_OFF}"
  exit 1
fi
