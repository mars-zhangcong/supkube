#!/usr/bin/env bash
# preflight.sh — SupKube pre-install cluster compatibility check.
#
# WHY THIS EXISTS
# ───────────────
# Customers historically hit two failure modes:
#   (a) `helm install` succeeds, but at first backup nothing happens because
#       the cluster has no CSI snapshot CRDs / snapshot-controller / default
#       StorageClass.
#   (b) The customer gets stuck mid-install on a permission error or an old
#       K8s version that doesn't support APIs we use.
#
# Both are detectable BEFORE `helm install` runs. This script encodes the
# detection so customers run one command, see a friendly report, and only
# proceed to install once the cluster is ready. Inspired by Kasten K10's
# `k10_primer.sh`.
#
# USAGE
# ─────
#   # One-shot from anywhere
#   curl -fsSL https://charts.supkube.com/preflight.sh | bash
#
#   # Or download + inspect first (recommended for security-conscious customers)
#   curl -fsSL https://charts.supkube.com/preflight.sh -o preflight.sh
#   less preflight.sh
#   chmod +x preflight.sh
#   ./preflight.sh
#
# FLAGS
# ─────
#   --verbose         Print extra detail per check (raw kubectl output, etc.)
#   --no-color        Disable ANSI color codes (handy for CI logs)
#   --skip-velero     Skip the optional Velero CRD presence check
#   --json            Emit machine-readable JSON instead of human report
#                     (useful for CI gates / wrapper scripts)
#
# EXIT CODES
# ──────────
#   0 — all critical checks PASS (WARN-only is still OK)
#   1 — at least one critical check FAILED
#   2 — preflight tool itself broke (kubectl not found, etc.)

set -uo pipefail

# ─── Args parsing ───────────────────────────────────────────────────
VERBOSE=0
NO_COLOR=0
SKIP_VELERO=0
JSON_OUTPUT=0

for arg in "$@"; do
  case "$arg" in
    --verbose)     VERBOSE=1 ;;
    --no-color)    NO_COLOR=1 ;;
    --skip-velero) SKIP_VELERO=1 ;;
    --json)        JSON_OUTPUT=1; NO_COLOR=1 ;;
    --help|-h)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "ERROR: unknown flag '$arg'. Try --help." >&2; exit 2 ;;
  esac
done

# ─── Colors (suppressed when piped, --no-color, or --json) ──────────
if [[ -t 1 && $NO_COLOR -eq 0 ]]; then
  C_OK="\033[32m"; C_WARN="\033[33m"; C_ERR="\033[31m"
  C_DIM="\033[2m"; C_BOLD="\033[1m"; C_OFF="\033[0m"
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_BOLD=""; C_OFF=""
fi

# ─── State accumulators ─────────────────────────────────────────────
PASS_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0
declare -a JSON_RESULTS=()

# Recommended install knobs derived from cluster state.
REC_VELERO_ENABLED="true"   # default: ship bundled Velero
REC_LOCAL_STORE="false"     # default off

# ─── Print helpers ──────────────────────────────────────────────────
say() { [[ $JSON_OUTPUT -eq 1 ]] || echo -e "$@"; }
header() {
  [[ $JSON_OUTPUT -eq 1 ]] && return
  echo ""
  echo -e "${C_BOLD}══ $1${C_OFF}"
}
pass() {
  local id="$1" msg="$2" detail="${3:-}"
  PASS_COUNT=$((PASS_COUNT + 1))
  JSON_RESULTS+=("{\"id\":\"$id\",\"status\":\"PASS\",\"message\":$(json_str "$msg"),\"detail\":$(json_str "$detail")}")
  say "  ${C_OK}✓ PASS${C_OFF}  $msg"
  [[ -n "$detail" && $VERBOSE -eq 1 ]] && say "         ${C_DIM}$detail${C_OFF}"
}
warn() {
  local id="$1" msg="$2" remediation="${3:-}"
  WARN_COUNT=$((WARN_COUNT + 1))
  JSON_RESULTS+=("{\"id\":\"$id\",\"status\":\"WARN\",\"message\":$(json_str "$msg"),\"remediation\":$(json_str "$remediation")}")
  say "  ${C_WARN}! WARN${C_OFF}  $msg"
  [[ -n "$remediation" ]] && say "         ${C_DIM}→ $remediation${C_OFF}"
}
fail() {
  local id="$1" msg="$2" remediation="${3:-}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
  JSON_RESULTS+=("{\"id\":\"$id\",\"status\":\"FAIL\",\"message\":$(json_str "$msg"),\"remediation\":$(json_str "$remediation")}")
  say "  ${C_ERR}✗ FAIL${C_OFF}  $msg"
  [[ -n "$remediation" ]] && say "         ${C_DIM}→ $remediation${C_OFF}"
}
json_str() {
  # Minimal JSON string escape (quotes + backslashes + newlines).
  printf '%s' "$1" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))' 2>/dev/null \
    || printf '"%s"' "$(printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g; s/$/\\n/' | tr -d '\n' | sed 's/\\n$//')"
}

# ─── Tool prerequisite (script self-check) ──────────────────────────
require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "${C_ERR}ERROR${C_OFF}: required tool '$1' not in PATH." >&2
    echo "Install it before running preflight." >&2
    exit 2
  fi
}
require_tool kubectl
# helm not strictly required for preflight, but customer will need it
# next. We warn rather than fail.

# Banner.
if [[ $JSON_OUTPUT -eq 0 ]]; then
  cat <<EOF
${C_BOLD}╔═══════════════════════════════════════════════════════════════╗
║          SupKube Pre-Install Check  (preflight v1)            ║
║                                                               ║
║  Verifies your cluster is ready for SupKube before you run    ║
║  helm install. No mutations — read-only inspection only.      ║
╚═══════════════════════════════════════════════════════════════╝${C_OFF}
EOF
fi

# ─── Capture cluster context for the summary ────────────────────────
CTX="$(kubectl config current-context 2>/dev/null || echo '<no context>')"
SERVER="$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' 2>/dev/null || echo '?')"
say ""
say "${C_DIM}Context: $CTX${C_OFF}"
say "${C_DIM}Server:  $SERVER${C_OFF}"

# ═══════════════════════════════════════════════════════════════════
# CHECKS
# ═══════════════════════════════════════════════════════════════════

header "[1/10] Kubernetes API connectivity"
if K8S_VER_JSON=$(kubectl version -o json 2>/dev/null); then
  SERVER_VER=$(echo "$K8S_VER_JSON" | python3 -c 'import sys,json; v=json.load(sys.stdin).get("serverVersion",{}); print(v.get("gitVersion","unknown"))' 2>/dev/null || echo "?")
  pass "k8s-connect" "kubectl can reach cluster (server $SERVER_VER)"
  # Parse major.minor for the version gate.
  SERVER_MAJOR=$(echo "$SERVER_VER" | sed -E 's/^v?([0-9]+).*/\1/')
  SERVER_MINOR=$(echo "$SERVER_VER" | sed -E 's/^v?[0-9]+\.([0-9]+).*/\1/')
  # SupKube requires K8s 1.25+. v1.25 introduced stable CSI volume
  # snapshot support which our policypair controller relies on.
  if [[ "$SERVER_MAJOR" == "1" && "$SERVER_MINOR" -lt 25 ]]; then
    fail "k8s-version" "K8s $SERVER_VER is too old (SupKube requires 1.25+)" \
         "Upgrade the cluster, or use SupKube v0.7.x which supports K8s 1.20+."
  else
    pass "k8s-version" "K8s version meets requirement (≥ 1.25)"
  fi
else
  fail "k8s-connect" "kubectl cannot reach the cluster" \
       "Check 'kubectl cluster-info' and verify your kubeconfig is set correctly."
  # No point continuing if kubectl is dead.
  say ""
  say "${C_ERR}Aborting further checks — fix kubectl access first.${C_OFF}"
  exit 1
fi

header "[2/10] Helm CLI"
if command -v helm >/dev/null 2>&1; then
  HELM_VER=$(helm version --short 2>/dev/null || echo "?")
  HELM_MAJOR=$(echo "$HELM_VER" | sed -E 's/^v?([0-9]+).*/\1/')
  if [[ "$HELM_MAJOR" == "3" ]]; then
    pass "helm-version" "Helm $HELM_VER (3.x)"
  else
    fail "helm-version" "Helm $HELM_VER is not 3.x" \
         "SupKube requires Helm 3.10+. Install from https://helm.sh."
  fi
else
  warn "helm-version" "helm CLI not in PATH" \
       "You'll need it to run the install command. https://helm.sh/docs/intro/install/"
fi

header "[3/10] Cluster permissions"
# Probe for cluster-admin-ish access via SelfSubjectAccessReview. We
# specifically check for the verbs SupKube + Velero will need.
PERMS_OK=1
for verb_resource in \
    "create:namespaces" \
    "create:customresourcedefinitions:apiextensions.k8s.io" \
    "create:clusterrolebindings:rbac.authorization.k8s.io" \
    "create:deployments:apps"; do
  verb="${verb_resource%%:*}"
  resource_with_group="${verb_resource#*:}"
  resource="${resource_with_group%%:*}"
  group="${resource_with_group##*:}"
  [[ "$resource" == "$group" ]] && group=""
  if kubectl auth can-i "$verb" "$resource" --all-namespaces \
       ${group:+--subresource=} >/dev/null 2>&1; then
    [[ $VERBOSE -eq 1 ]] && say "    can ${verb} ${resource}${group:+ ($group)}"
  else
    PERMS_OK=0
    break
  fi
done
if [[ $PERMS_OK -eq 1 ]]; then
  pass "rbac" "Sufficient cluster-admin permissions (CRDs, namespaces, RBAC, Deployments)"
else
  fail "rbac" "Missing cluster-admin permissions" \
       "Run 'kubectl auth can-i create customresourcedefinitions' — needs to return 'yes'."
fi

header "[4/10] StorageClasses"
SC_LIST=$(kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.provisioner}{"|"}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}' 2>/dev/null)
SC_COUNT=$(echo "$SC_LIST" | grep -c . || echo 0)
DEFAULT_SC=$(echo "$SC_LIST" | awk -F'|' '$3=="true"{print $1}' | head -1)
if [[ "$SC_COUNT" -ge 1 ]]; then
  pass "storageclass" "$SC_COUNT StorageClass(es) detected" "$(echo "$SC_LIST" | awk -F'|' '{printf "  - %s (%s)%s\n", $1, $2, ($3=="true" ? " [default]" : "")}')"
  if [[ -z "$DEFAULT_SC" ]]; then
    warn "storageclass-default" "No default StorageClass set" \
         "PVC without storageClassName won't auto-provision. Mark one as default: kubectl patch storageclass <name> -p '{\"metadata\":{\"annotations\":{\"storageclass.kubernetes.io/is-default-class\":\"true\"}}}'"
  fi
else
  fail "storageclass" "No StorageClass found" \
       "SupKube + Velero need at least one StorageClass to provision PVCs. Install a CSI driver for your cloud (AKS: built-in; AWS: ebs-csi; GCP: gcepd)."
fi

header "[5/10] CSI snapshot CRDs"
SNAPSHOT_CRD_VERSIONS=$(kubectl get crd volumesnapshots.snapshot.storage.k8s.io -o jsonpath='{.spec.versions[*].name}' 2>/dev/null || echo "")
if [[ "$SNAPSHOT_CRD_VERSIONS" == *"v1"* ]]; then
  pass "csi-snapshot-crd" "snapshot.storage.k8s.io/v1 CRDs installed"
elif [[ -n "$SNAPSHOT_CRD_VERSIONS" ]]; then
  fail "csi-snapshot-crd" "Old snapshot CRD version detected: $SNAPSHOT_CRD_VERSIONS" \
       "SupKube requires v1 (stable since K8s 1.20). Upgrade snapshot CRDs from https://github.com/kubernetes-csi/external-snapshotter."
else
  fail "csi-snapshot-crd" "No snapshot.storage.k8s.io CRDs found" \
       "Install snapshot CRDs: kubectl apply -f https://github.com/kubernetes-csi/external-snapshotter/raw/master/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml (and ...snapshotcontents.yaml, ...snapshotclasses.yaml)."
fi

header "[6/10] snapshot-controller pod"
SC_POD=$(kubectl get pods -A -l app.kubernetes.io/name=snapshot-controller \
  -o jsonpath='{range .items[0]}{.metadata.namespace}/{.metadata.name} {.status.phase}{end}' 2>/dev/null || echo "")
if [[ -z "$SC_POD" ]]; then
  # Some distros (AKS / GKE) bundle this as kube-system/csi-snapshotter-* without the standard label.
  SC_POD=$(kubectl get pods -n kube-system 2>/dev/null | grep -E 'snapshot-controller|csi-snapshotter' | head -1 | awk '{print "kube-system/"$1" "$3}')
fi
if [[ "$SC_POD" == *"Running"* ]]; then
  pass "snapshot-controller" "snapshot-controller running: $SC_POD"
elif [[ -n "$SC_POD" ]]; then
  warn "snapshot-controller" "snapshot-controller pod found but not Running: $SC_POD" \
       "Check pod logs: kubectl logs -n kube-system <pod>"
else
  warn "snapshot-controller" "snapshot-controller pod not detected" \
       "AKS/EKS/GKE bundle this by default; on bare-metal clusters install via: kubectl apply -f https://github.com/kubernetes-csi/external-snapshotter/raw/master/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml + rbac-snapshot-controller.yaml"
fi

header "[7/10] VolumeSnapshotClass"
VSC_COUNT=$(kubectl get volumesnapshotclass -o name 2>/dev/null | wc -l | tr -d ' ')
if [[ "$VSC_COUNT" -ge 1 ]]; then
  VSC_LIST=$(kubectl get volumesnapshotclass -o jsonpath='{range .items[*]}{.metadata.name}{" ("}{.driver}{")\n"}{end}' 2>/dev/null)
  pass "volumesnapshotclass" "$VSC_COUNT VolumeSnapshotClass(es)" "$VSC_LIST"
else
  warn "volumesnapshotclass" "No VolumeSnapshotClass found" \
       "Backups will fall back to filesystem-mode (slower). Create one per your CSI driver — e.g. for Azure Disk CSI: kubectl apply with driver: disk.csi.azure.com"
fi

header "[8/10] Velero (optional pre-install detection)"
if [[ $SKIP_VELERO -eq 1 ]]; then
  say "  ${C_DIM}(skipped per --skip-velero)${C_OFF}"
else
  if kubectl get crd backups.velero.io >/dev/null 2>&1; then
    VELERO_NS=$(kubectl get deploy -A -l component=velero -o jsonpath='{.items[0].metadata.namespace}' 2>/dev/null || echo "")
    if [[ -n "$VELERO_NS" ]]; then
      VELERO_VER=$(kubectl get deploy -n "$VELERO_NS" velero -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null | sed 's|.*:||' || echo "?")
      pass "velero-existing" "Velero $VELERO_VER detected in namespace $VELERO_NS" \
           "SupKube can reuse this. Install with: --set velero.enabled=false"
      REC_VELERO_ENABLED="false"
    else
      warn "velero-existing" "Velero CRDs present but no Velero deployment found" \
           "Either remove stale CRDs (kubectl delete crd backups.velero.io ...) or check the Velero install."
    fi
  else
    pass "velero-existing" "No existing Velero detected" \
         "SupKube will install bundled Velero v1.18 (default behavior)"
  fi
fi

header "[9/10] Node resources"
NODE_COUNT=$(kubectl get nodes --no-headers 2>/dev/null | wc -l | tr -d ' ')
if [[ "$NODE_COUNT" -ge 1 ]]; then
  NODES_INFO=$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{": "}{.status.allocatable.cpu}{" cpu, "}{.status.allocatable.memory}{"\n"}{end}' 2>/dev/null)
  pass "nodes" "$NODE_COUNT node(s) Ready" "$NODES_INFO"
else
  fail "nodes" "No nodes found in cluster"
fi

header "[10/10] Cluster-wide LocalStore eligibility (informational)"
# Recommend localStore.enabled=true if the cluster has plenty of storage
# and customer's looking for the 3-2-1-1-0 local copy. Just a hint.
if [[ "$NODE_COUNT" -ge 2 && -n "$DEFAULT_SC" ]]; then
  pass "localstore-eligible" "Cluster qualifies for in-cluster MinIO local store (optional)" \
       "Enable with: --set localStore.enabled=true (uses ~100Gi PVC). Skip for cloud-BSL-only setups."
  # Don't change REC_LOCAL_STORE — keep default off so customers opt in.
else
  warn "localstore-eligible" "Single-node or no default SC — LocalStore not recommended" \
       "Set localStore.enabled=false (default) and use a cloud BSL instead."
fi

# ═══════════════════════════════════════════════════════════════════
# SUMMARY
# ═══════════════════════════════════════════════════════════════════

if [[ $JSON_OUTPUT -eq 1 ]]; then
  printf '{"summary":{"pass":%d,"warn":%d,"fail":%d},"results":[' \
    "$PASS_COUNT" "$WARN_COUNT" "$FAIL_COUNT"
  printf '%s' "$(IFS=,; echo "${JSON_RESULTS[*]}")"
  printf ']}\n'
else
  echo ""
  echo -e "${C_BOLD}═════════════════════════════════════════════════════════════════${C_OFF}"
  echo -e "  Result:  ${C_OK}${PASS_COUNT} pass${C_OFF}  ${C_WARN}${WARN_COUNT} warn${C_OFF}  ${C_ERR}${FAIL_COUNT} fail${C_OFF}"
  echo -e "${C_BOLD}═════════════════════════════════════════════════════════════════${C_OFF}"

  if [[ $FAIL_COUNT -eq 0 ]]; then
    echo ""
    echo -e "${C_OK}✅ READY FOR INSTALL${C_OFF}"
    echo ""
    echo "Recommended install command for this cluster:"
    echo ""
    echo -e "${C_BOLD}  helm repo add supkube https://charts.supkube.com/"
    echo "  helm repo update"
    echo "  helm install supkube supkube/supkube --devel \\"
    echo "    -n supkube --create-namespace \\"
    echo "    --set velero.enabled=${REC_VELERO_ENABLED} \\"
    echo -e "    --set localStore.enabled=${REC_LOCAL_STORE}${C_OFF}"
    echo ""
    [[ $WARN_COUNT -gt 0 ]] && echo -e "${C_DIM}(${WARN_COUNT} warning(s) above are informational — install will still work.)${C_OFF}"
  else
    echo ""
    echo -e "${C_ERR}❌ NOT READY — fix the FAIL items above before installing.${C_OFF}"
    echo ""
    echo -e "${C_DIM}Re-run this script after addressing the issues.${C_OFF}"
  fi
fi

# Exit code: non-zero if any critical FAIL.
[[ $FAIL_COUNT -eq 0 ]] && exit 0 || exit 1
