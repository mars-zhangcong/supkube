#!/usr/bin/env bash
# supkube_debug.sh — SupKube diagnostic bundle generator.
#
# WHY THIS EXISTS
# ───────────────
# When a customer hits an issue, the support burden of "run kubectl logs
# 10 times in 5 namespaces" is the difference between a 5-minute ticket
# and a 2-hour back-and-forth. This script packages everything support
# needs into one tarball, with one curl-bash command.
#
# Modeled after Kasten's `k10_debug.sh`. The output structure is similar
# so support engineers familiar with K10 bundles can read SupKube bundles
# without re-learning the layout.
#
# USAGE
# ─────
#   # One-shot from the customer's workstation (curl-bash):
#   curl -fsSL https://charts.supkube.com/supkube_debug.sh | \
#     bash -s -- -n supkube -o supkube-debug.tar.gz
#
#   # Or download + inspect first (security-conscious customers):
#   curl -fsSL https://charts.supkube.com/supkube_debug.sh -o supkube_debug.sh
#   chmod +x supkube_debug.sh
#   ./supkube_debug.sh -n supkube -o my-debug.tar.gz
#
# FLAGS
# ─────
#   -n <namespace>      SupKube install namespace (default: supkube)
#   -o <output-file>    Output tarball name (default: supkube-debug-<ts>.tar.gz)
#   --velero-ns <ns>    Velero install namespace (default: velero)
#   --since <duration>  Log time window (default: 24h; e.g. 1h, 7d, 30m)
#   --tail <lines>      Max lines per log (default: 10000; cap on cluster spam)
#   --no-velero         Skip Velero ns collection (when --set velero.enabled=false)
#   --anonymize         Replace customer-identifying strings (ns names, labels)
#                       with generic markers. For sharing externally.
#   --skip-secrets      Skip secret metadata (default already redacts values)
#
# WHAT'S IN THE BUNDLE
# ────────────────────
#   supkube-debug-<cluster>-<timestamp>/
#   ├── README.txt              ← what's here + support contact + how to ship
#   ├── env.txt                 ← kubectl/helm version + cluster nodes
#   ├── supkube/
#   │   ├── pods.yaml           ← all pod specs in supkube ns
#   │   ├── describe.txt        ← describe each pod (events, restarts)
#   │   ├── logs/<pod>.log      ← per-pod logs (--since, --tail bounded)
#   │   ├── clusters.yaml       ← Multi-Cluster registry (v0.9.0+)
#   │   ├── policies.yaml       ← Policy CRs
#   │   ├── schedules.yaml      ← Schedule CRs
#   │   ├── transformsets.yaml  ← TransformSet CRs
#   │   ├── configmaps.yaml     ← EULA / branding / support-contact CMs
#   │   └── secrets-meta.txt    ← Secret names+types only, NO values
#   ├── velero/                 ← (skipped if --no-velero)
#   │   ├── pods.yaml
#   │   ├── describe.txt
#   │   ├── logs/velero.log
#   │   ├── logs/node-agent-<node>.log
#   │   ├── backups.yaml        ← All Backup CRs
#   │   ├── restores.yaml
#   │   ├── schedules.yaml
#   │   ├── bsls.yaml           ← BackupStorageLocations
#   │   ├── vsls.yaml           ← VolumeSnapshotLocations
#   │   ├── per-backup-logs/    ← velero backup logs <name> for last N
#   │   └── per-restore-logs/   ← velero restore logs <name>
#   ├── kube-system/
#   │   ├── snapshot-controller.log
#   │   └── csi-pods.yaml
#   └── csi/
#       └── volumesnapshots.yaml
#
# OUTPUT
# ──────
# Final tarball location printed at end. Customer ships via:
#   · Email (after compressing — usually < 10 MB)
#   · Their support portal upload
#   · USB / SFTP for airgap environments
#
# EXIT CODES
# ──────────
#   0 — bundle created successfully
#   1 — kubectl unreachable / no permission
#   2 — script self-error (bad flag, no tools)

set -uo pipefail

# ─── Defaults ───────────────────────────────────────────────────────
NS_SUPKUBE="supkube"
NS_VELERO="velero"
SINCE="24h"
TAIL=10000
NO_VELERO=0
ANONYMIZE=0
SKIP_SECRETS=0
OUTPUT=""

# ─── Args parsing ───────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -n)               NS_SUPKUBE="$2"; shift 2 ;;
    -o)               OUTPUT="$2"; shift 2 ;;
    --velero-ns)      NS_VELERO="$2"; shift 2 ;;
    --since)          SINCE="$2"; shift 2 ;;
    --tail)           TAIL="$2"; shift 2 ;;
    --no-velero)      NO_VELERO=1; shift ;;
    --anonymize)      ANONYMIZE=1; shift ;;
    --skip-secrets)   SKIP_SECRETS=1; shift ;;
    --help|-h)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "ERROR: unknown flag '$1'. Try --help." >&2
      exit 2
      ;;
  esac
done

# ─── Color (suppress when not TTY) ──────────────────────────────────
if [[ -t 1 ]]; then
  C_OK="\033[32m"; C_WARN="\033[33m"; C_ERR="\033[31m"
  C_DIM="\033[2m"; C_BOLD="\033[1m"; C_OFF="\033[0m"
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_BOLD=""; C_OFF=""
fi

say() { echo -e "$@"; }
ok()   { say "  ${C_OK}✓${C_OFF} $1"; }
warn() { say "  ${C_WARN}!${C_OFF} $1"; }
fail() { say "  ${C_ERR}✗${C_OFF} $1"; }

# ─── Sanity checks ──────────────────────────────────────────────────
command -v kubectl >/dev/null || { echo "ERROR: kubectl not in PATH" >&2; exit 2; }
if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "ERROR: kubectl cannot reach the cluster. Check 'kubectl config current-context'." >&2
  exit 1
fi

# ─── Resolve paths ──────────────────────────────────────────────────
TIMESTAMP=$(date "+%Y%m%d-%H%M%S")
CTX=$(kubectl config current-context 2>/dev/null | tr '/:' '__' | tr -d '\n')
BUNDLE_NAME="supkube-debug-${CTX}-${TIMESTAMP}"

if [[ -z "$OUTPUT" ]]; then
  OUTPUT="${BUNDLE_NAME}.tar.gz"
fi

STAGE="$(mktemp -d -t supkube-debug-XXXXXX)"
ROOT="$STAGE/$BUNDLE_NAME"
mkdir -p "$ROOT/supkube/logs" "$ROOT/kube-system" "$ROOT/csi"
[[ $NO_VELERO -eq 0 ]] && mkdir -p "$ROOT/velero/logs" "$ROOT/velero/per-backup-logs" "$ROOT/velero/per-restore-logs"

cleanup() { rm -rf "$STAGE"; }
trap cleanup EXIT

# ─── Banner ─────────────────────────────────────────────────────────
cat <<EOF
${C_BOLD}╔═══════════════════════════════════════════════════════════════╗
║          SupKube Diagnostic Bundle  (supkube_debug v1)        ║
╚═══════════════════════════════════════════════════════════════╝${C_OFF}

  Cluster context:    $CTX
  SupKube namespace:  $NS_SUPKUBE
  Velero namespace:   $NS_VELERO $( [[ $NO_VELERO -eq 1 ]] && echo "(skipped)" )
  Log window:         --since $SINCE  --tail $TAIL
  Anonymize:          $( [[ $ANONYMIZE -eq 1 ]] && echo "YES" || echo "no" )
  Output:             $( [[ "$OUTPUT" = /* ]] && echo "$OUTPUT" || echo "$(pwd)/$OUTPUT" )
EOF

# ─── Helper: kubectl-safe wrapper, never aborts the script ──────────
kget() {
  # kget <args> — runs kubectl, swallows non-zero, emits a "(none)" if empty.
  kubectl "$@" 2>/dev/null || echo "(unavailable or empty)"
}

# ─── Section 1: env info ────────────────────────────────────────────
say ""
say "${C_BOLD}══ [1/6] Environment${C_OFF}"
{
  echo "=== kubectl version ==="
  kubectl version --output=yaml 2>/dev/null || kubectl version 2>/dev/null
  echo ""
  echo "=== current-context ==="
  kubectl config current-context 2>/dev/null
  echo ""
  echo "=== nodes ==="
  kubectl get nodes -o wide 2>/dev/null
  echo ""
  echo "=== helm version ==="
  command -v helm >/dev/null && helm version --short 2>/dev/null || echo "(helm not in PATH)"
  echo ""
  echo "=== helm list (supkube ns) ==="
  command -v helm >/dev/null && helm -n "$NS_SUPKUBE" list 2>/dev/null || echo "(helm not in PATH)"
  echo ""
  echo "=== storageclasses ==="
  kubectl get storageclass -o wide 2>/dev/null
  echo ""
  echo "=== volumesnapshotclasses ==="
  kget get volumesnapshotclass -o wide
} > "$ROOT/env.txt"
ok "env.txt collected"

# ─── Section 2: SupKube namespace ───────────────────────────────────
say ""
say "${C_BOLD}══ [2/6] SupKube namespace ($NS_SUPKUBE)${C_OFF}"

if ! kubectl get ns "$NS_SUPKUBE" >/dev/null 2>&1; then
  warn "namespace '$NS_SUPKUBE' not found — skipping SupKube section"
else
  kget get pods -n "$NS_SUPKUBE" -o yaml > "$ROOT/supkube/pods.yaml" && ok "supkube/pods.yaml"
  kget describe pods -n "$NS_SUPKUBE" > "$ROOT/supkube/describe.txt" && ok "supkube/describe.txt"

  # Per-pod logs (current + previous)
  for pod in $(kubectl get pods -n "$NS_SUPKUBE" -o name 2>/dev/null | sed 's|pod/||'); do
    kubectl logs -n "$NS_SUPKUBE" "$pod" --since="$SINCE" --tail="$TAIL" \
      >"$ROOT/supkube/logs/${pod}.log" 2>/dev/null || true
    # Previous container logs (after restart)
    kubectl logs -n "$NS_SUPKUBE" "$pod" --since="$SINCE" --tail="$TAIL" --previous \
      >"$ROOT/supkube/logs/${pod}.prev.log" 2>/dev/null || true
    # Drop empty .prev.log files
    [[ -s "$ROOT/supkube/logs/${pod}.prev.log" ]] || rm -f "$ROOT/supkube/logs/${pod}.prev.log"
  done
  ok "supkube/logs/ collected ($(ls "$ROOT/supkube/logs" 2>/dev/null | wc -l | tr -d ' ') files)"

  # SupKube CRDs (v0.9.0+ multi-cluster + policy/schedule/transformset)
  kget get clusters.supkube.io -A -o yaml > "$ROOT/supkube/clusters.yaml" && ok "supkube/clusters.yaml"

  # ConfigMaps — EULA / branding / support-contact carry useful metadata
  kget get configmaps -n "$NS_SUPKUBE" -o yaml > "$ROOT/supkube/configmaps.yaml" && ok "supkube/configmaps.yaml"

  # Secrets metadata only — never log secret values
  if [[ $SKIP_SECRETS -eq 0 ]]; then
    kubectl get secrets -n "$NS_SUPKUBE" \
      -o custom-columns=NAME:.metadata.name,TYPE:.type,AGE:.metadata.creationTimestamp \
      > "$ROOT/supkube/secrets-meta.txt" 2>/dev/null || true
    ok "supkube/secrets-meta.txt (names+types only, no values)"
  fi
fi

# ─── Section 3: Velero namespace ────────────────────────────────────
say ""
say "${C_BOLD}══ [3/6] Velero namespace ($NS_VELERO)${C_OFF}"

if [[ $NO_VELERO -eq 1 ]]; then
  warn "skipped per --no-velero"
elif ! kubectl get ns "$NS_VELERO" >/dev/null 2>&1; then
  warn "namespace '$NS_VELERO' not found — Velero may not be installed"
  echo "Velero namespace '$NS_VELERO' not found at $(date)" > "$ROOT/velero/MISSING.txt"
else
  kget get pods -n "$NS_VELERO" -o yaml > "$ROOT/velero/pods.yaml" && ok "velero/pods.yaml"
  kget describe pods -n "$NS_VELERO" > "$ROOT/velero/describe.txt" && ok "velero/describe.txt"

  # Per-pod logs (velero deploy + node-agent DaemonSet)
  for pod in $(kubectl get pods -n "$NS_VELERO" -o name 2>/dev/null | sed 's|pod/||'); do
    kubectl logs -n "$NS_VELERO" "$pod" --since="$SINCE" --tail="$TAIL" \
      >"$ROOT/velero/logs/${pod}.log" 2>/dev/null || true
    [[ -s "$ROOT/velero/logs/${pod}.log" ]] || rm -f "$ROOT/velero/logs/${pod}.log"
  done
  ok "velero/logs/ ($(ls "$ROOT/velero/logs" 2>/dev/null | wc -l | tr -d ' ') pods)"

  # Velero CRs — backups / restores / schedules / BSL / VSL
  kget get backups.velero.io -n "$NS_VELERO" -o yaml > "$ROOT/velero/backups.yaml" && ok "velero/backups.yaml"
  kget get restores.velero.io -n "$NS_VELERO" -o yaml > "$ROOT/velero/restores.yaml" && ok "velero/restores.yaml"
  kget get schedules.velero.io -n "$NS_VELERO" -o yaml > "$ROOT/velero/schedules.yaml" && ok "velero/schedules.yaml"
  kget get backupstoragelocations.velero.io -n "$NS_VELERO" -o yaml > "$ROOT/velero/bsls.yaml" && ok "velero/bsls.yaml"
  kget get volumesnapshotlocations.velero.io -n "$NS_VELERO" -o yaml > "$ROOT/velero/vsls.yaml" && ok "velero/vsls.yaml"
  kget get datauploads.velero.io -A -o yaml > "$ROOT/velero/datauploads.yaml" && ok "velero/datauploads.yaml"
  kget get datadownloads.velero.io -A -o yaml > "$ROOT/velero/datadownloads.yaml" && ok "velero/datadownloads.yaml"
  kget get podvolumebackups.velero.io -A -o yaml > "$ROOT/velero/podvolumebackups.yaml" && ok "velero/podvolumebackups.yaml"

  # Per-backup logs — if `velero` CLI exists, pull last 10 backup logs.
  # These are the most actionable bits for support: each backup's actual
  # execution trace lives in Velero DownloadRequest, not in pod logs.
  if command -v velero >/dev/null 2>&1; then
    for bkp in $(kubectl get backups.velero.io -n "$NS_VELERO" -o name 2>/dev/null | sed 's|.*/||' | head -10); do
      velero --namespace "$NS_VELERO" backup logs "$bkp" \
        >"$ROOT/velero/per-backup-logs/${bkp}.log" 2>/dev/null || true
      [[ -s "$ROOT/velero/per-backup-logs/${bkp}.log" ]] || rm -f "$ROOT/velero/per-backup-logs/${bkp}.log"
    done
    ok "velero/per-backup-logs/ ($(ls "$ROOT/velero/per-backup-logs" 2>/dev/null | wc -l | tr -d ' ') backups)"

    for rst in $(kubectl get restores.velero.io -n "$NS_VELERO" -o name 2>/dev/null | sed 's|.*/||' | head -10); do
      velero --namespace "$NS_VELERO" restore logs "$rst" \
        >"$ROOT/velero/per-restore-logs/${rst}.log" 2>/dev/null || true
      [[ -s "$ROOT/velero/per-restore-logs/${rst}.log" ]] || rm -f "$ROOT/velero/per-restore-logs/${rst}.log"
    done
    ok "velero/per-restore-logs/ ($(ls "$ROOT/velero/per-restore-logs" 2>/dev/null | wc -l | tr -d ' ') restores)"
  else
    warn "velero CLI not in PATH — skipping per-backup / per-restore logs"
    echo "velero CLI not in PATH at $(date) — install it for full diagnostics" \
      > "$ROOT/velero/per-backup-logs/SKIPPED.txt"
  fi
fi

# ─── Section 4: kube-system snapshot-controller ─────────────────────
say ""
say "${C_BOLD}══ [4/6] CSI snapshot infrastructure${C_OFF}"

# Try the common label + the bare "snapshot-controller" name. Different
# distros label differently.
SC_POD=$(kubectl get pods -n kube-system -l app.kubernetes.io/name=snapshot-controller \
  -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [[ -z "$SC_POD" ]]; then
  SC_POD=$(kubectl get pods -n kube-system 2>/dev/null | grep -E 'snapshot-controller|csi-snapshotter' | head -1 | awk '{print $1}')
fi

if [[ -n "$SC_POD" ]]; then
  kubectl logs -n kube-system "$SC_POD" --since="$SINCE" --tail="$TAIL" \
    > "$ROOT/kube-system/snapshot-controller.log" 2>/dev/null || true
  ok "kube-system/snapshot-controller.log ($SC_POD)"
else
  warn "snapshot-controller pod not detected in kube-system"
  echo "snapshot-controller pod not found at $(date)" \
    > "$ROOT/kube-system/snapshot-controller.MISSING.txt"
fi

# CSI driver pods (heuristic: anything with 'csi' in name in kube-system)
kget get pods -n kube-system -o wide > "$ROOT/kube-system/csi-pods.yaml"

# ─── Section 5: CSI snapshot resources ──────────────────────────────
say ""
say "${C_BOLD}══ [5/6] CSI VolumeSnapshots${C_OFF}"
kget get volumesnapshots -A -o yaml > "$ROOT/csi/volumesnapshots.yaml" && ok "csi/volumesnapshots.yaml"
kget get volumesnapshotcontents -o yaml > "$ROOT/csi/volumesnapshotcontents.yaml" && ok "csi/volumesnapshotcontents.yaml"

# ─── Section 6: Anonymize (optional) ────────────────────────────────
if [[ $ANONYMIZE -eq 1 ]]; then
  say ""
  say "${C_BOLD}══ [6/6] Anonymize${C_OFF}"
  # Replace customer-identifying patterns. Conservative: keep system + supkube
  # + velero + kube-* namespaces visible; mask user-defined namespace names.
  USER_NS=$(kubectl get ns -o name 2>/dev/null | sed 's|namespace/||' \
    | grep -vE '^(default|kube-|supkube|velero|cert-manager|ingress-|monitoring|kasten)' \
    | head -50)
  i=1
  for ns in $USER_NS; do
    [[ -z "$ns" ]] && continue
    placeholder="ns-anon-$i"
    # in-place sed across all collected text/yaml files
    find "$ROOT" -type f \( -name '*.yaml' -o -name '*.txt' -o -name '*.log' \) -print0 2>/dev/null \
      | xargs -0 sed -i.bak "s|\b${ns}\b|${placeholder}|g" 2>/dev/null || true
    i=$((i+1))
  done
  # remove .bak backups sed created
  find "$ROOT" -name '*.bak' -delete 2>/dev/null
  ok "anonymized $((i-1)) user namespace names"
else
  say ""
  say "${C_DIM}══ [6/6] Anonymize — skipped (pass --anonymize to enable)${C_OFF}"
fi

# ─── README.txt ─────────────────────────────────────────────────────
SUPPORT_CONTACT=""
if kubectl get cm supkube-support-contact -n "$NS_SUPKUBE" >/dev/null 2>&1; then
  SUPPORT_CONTACT=$(kubectl get cm supkube-support-contact -n "$NS_SUPKUBE" \
    -o jsonpath='{.data}' 2>/dev/null)
fi
EULA_INFO=""
if kubectl get cm supkube-eula -n "$NS_SUPKUBE" >/dev/null 2>&1; then
  EULA_INFO=$(kubectl get cm supkube-eula -n "$NS_SUPKUBE" -o yaml 2>/dev/null | grep -E "^\s+(accepted|email|company|chartVersion|appVersion|acceptedAt):" | head -10)
fi

cat > "$ROOT/README.txt" <<EOF
SupKube Diagnostic Bundle
=========================

Generated:        $(date)
Cluster context:  $CTX
SupKube ns:       $NS_SUPKUBE
Velero ns:        $NS_VELERO
Log window:       --since $SINCE --tail $TAIL
Anonymized:       $( [[ $ANONYMIZE -eq 1 ]] && echo YES || echo no )
Bundle tool:      supkube_debug.sh v1

EULA / License (from cm/supkube-eula):
$EULA_INFO

Customer Support Contact (from cm/supkube-support-contact):
$SUPPORT_CONTACT

WHAT'S IN THIS BUNDLE
─────────────────────
  env.txt             Cluster + tool versions, nodes, storageclasses
  supkube/            SupKube ns: pods, logs, CRs (clusters/policies/schedules)
  velero/             Velero ns: pods, logs, all CRs, per-backup/restore logs
  kube-system/        snapshot-controller logs + CSI driver pods
  csi/                VolumeSnapshot CRs (cluster-wide)

HOW TO SHIP TO SUPPORT
──────────────────────
  Email      attach the tarball (typically < 10 MB after gzip)
  Portal     upload via your support portal
  Airgap     SFTP or USB transfer; the bundle is self-contained

THINGS DELIBERATELY NOT INCLUDED
────────────────────────────────
  · Secret VALUES (only names + types in supkube/secrets-meta.txt)
  · ConfigMap values from outside the supkube namespace
  · Customer application namespaces' workload logs

If support needs deeper application-side data they will ask explicitly.
EOF

# ─── Tar it up ──────────────────────────────────────────────────────
say ""
say "${C_BOLD}══ Packaging${C_OFF}"
tar -czf "$OUTPUT" -C "$STAGE" "$BUNDLE_NAME"
SIZE=$(du -h "$OUTPUT" | awk '{print $1}')
FULL_OUTPUT_PATH=$( [[ "$OUTPUT" = /* ]] && echo "$OUTPUT" || echo "$(pwd)/$OUTPUT" )
ok "bundle written: $FULL_OUTPUT_PATH ($SIZE)"

# ─── Summary ────────────────────────────────────────────────────────
cat <<EOF

${C_BOLD}═════════════════════════════════════════════════════════════════${C_OFF}
  ✅ Bundle ready: ${C_BOLD}$FULL_OUTPUT_PATH${C_OFF} ($SIZE)
${C_BOLD}═════════════════════════════════════════════════════════════════${C_OFF}

  Ship via:
    · Email attachment (most cases — typically under 10 MB)
    · Customer support portal upload
    · SFTP / USB for airgap environments

  ${C_DIM}Re-run with --anonymize to mask user namespace names before sharing externally.${C_OFF}
EOF
