#!/usr/bin/env bash
# run-b.sh — STAGED one-command trigger for route B (AKS dev -> AKS test).
# Walks Gate 0 -> Gate 2. STOPS at each gate for the operator to confirm.
# Refuses to run until preflight-b.sh passes. Only creates ISOLATED test
# resources (dr-test ns, test-* backups, our own ImportPolicy/Transforms).
# NEVER touches existing aks-app-backup-* / other namespaces / Kasten.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
SRC_CTX="${SRC_CTX:-aks-jumborca-dev}"   # source / backup origin
DST_CTX="${DST_CTX:-aks-jumborca-test}"  # target / import + restore
NS=dr-test
BK="test-dr-loop-$(date +%Y%m%d%H%M 2>/dev/null || echo MANUAL)"

pause(){ echo; echo ">>> GATE $1 reached. Review evidence above, then press ENTER to continue or Ctrl-C to stop."; read -r _; }

echo "### Route B: $SRC_CTX (source) -> $DST_CTX (target). Backup=$BK"
echo "### Step 0: preflight gate"
"$HERE/preflight-b.sh" || { echo "ABORT: preflight blocked. See 等待决策.md."; exit 1; }

# --- Gate 0 (smoke): prove ImportPolicy lists Azure + fingerprint enforce ---
echo "### Gate 0a: ImportPolicy run-once lists+imports from shared Azure BSL"
echo "### Gate 0b: tamper a fingerprint copy in an ISOLATED test prefix -> enforce must reject"
echo "    (manual: see README-runbook.md Gate 0 — uses test-* backup only)"
pause 0

# --- Gate 1: seed app + fs-backup on SOURCE (dev) ---
echo "### Gate 1: create isolated app on $SRC_CTX and fs-backup to Azure"
kubectl --context "$SRC_CTX" apply -f "$ROOT/manifests/00-namespace.yaml"
kubectl --context "$SRC_CTX" apply -f "$ROOT/manifests/10-postgres.yaml"
kubectl --context "$SRC_CTX" apply -f "$ROOT/manifests/20-adminer.yaml"
kubectl --context "$SRC_CTX" -n "$NS" rollout status statefulset/postgres --timeout=180s
kubectl --context "$SRC_CTX" apply -f "$ROOT/manifests/30-seed-5rows.yaml"
kubectl --context "$SRC_CTX" -n "$NS" wait --for=condition=complete job/seed-5rows --timeout=120s
SRC_CTX="$SRC_CTX" CTX="$SRC_CTX" "$HERE/verify-count.sh" 5
# fs-backup (Kopia), NOT csi snapshot — defaultVolumesToFsBackup=true
velero backup create "$BK" --include-namespaces "$NS" \
  --default-volumes-to-fs-backup --snapshot-move-data=false \
  --kubeconfig <(kubectl --context "$SRC_CTX" config view --raw --minify) || true
echo "    poll: velero backup describe $BK   (must reach Completed; fingerprint.json must land in BSL)"
pause 1

# --- Gate 2: import on target (test) + 3 transforms + restore + LB ---
echo "### Gate 2: import on $DST_CTX, apply transforms, restore, expose"
kubectl --context "$DST_CTX" apply -f "$ROOT/transforms/sc-remap-fwd.yaml"
kubectl --context "$DST_CTX" apply -f "$ROOT/transforms/lb-to-clusterip.yaml"   # only if target lacks LB; AKS has LB so optional
kubectl --context "$DST_CTX" apply -f "$ROOT/importpolicy/import-fwd-dev-to-test.yaml"
echo "    poll: kubectl --context $DST_CTX -n supkube get importpolicy import-fwd-dev-to-test -o yaml (status.imported should show $BK)"
echo "    then: create a Restore referencing the imported backup + TransformSet (see runbook), and verify:"
echo "          verify-count.sh 5 / verify-pgsha.sh / verify-fingerprint.sh / verify-ns-not-terminating.sh"
pause 2
echo "### B Gate 0-2 walked. Gate 3-4 (AKS new 5 rows + reverse import to dev) — see runbook."
