#!/usr/bin/env bash
# preflight-b.sh — HARD gate before route B (AKS dev -> AKS test) may run.
# Checks the 4 blockers found 2026-06-03. Exit 0 only if ALL pass.
# READ-ONLY. Pin contexts. Source cluster = dev, target = test.
set -uo pipefail

SRC_CTX="${SRC_CTX:-aks-jumborca-dev}"
DST_CTX="${DST_CTX:-aks-jumborca-test}"
SA="${SA:-rnd7sa71}"
CONTAINER="${CONTAINER:-velero-dr}"
FAIL=0
ok(){ echo "  ✓ $1"; }
no(){ echo "  ✗ $1"; FAIL=1; }

echo "== B-1: my Azure identity has Storage Blob Data Reader on $SA =="
if az storage blob list --account-name "$SA" --container-name "$CONTAINER" \
     --auth-mode login --num-results 1 -o none 2>/dev/null; then
  ok "storage blob list works (reader granted)"
else
  no "storage blob list DENIED — need 'Storage Blob Data Reader' on $SA (verify手段3 impossible without it). See 等待决策 D-WAIT-004."
fi

echo "== B-2: target Velero is HEALTHY on $DST_CTX =="
ph=$(kubectl --context "$DST_CTX" -n supkube get pod -l deploy=velero \
       -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
if [ "$ph" = "Running" ]; then ok "velero pod Running"; else
  no "velero pod not Running (got '${ph:-none}'; was Init:0/2 38h). See D-WAIT-005."
fi

echo "== B-3: target BSL azure-blob is Available on $DST_CTX =="
bph=$(kubectl --context "$DST_CTX" -n supkube get bsl azure-blob \
       -o jsonpath='{.status.phase}' 2>/dev/null)
if [ "$bph" = "Available" ]; then ok "BSL Available"; else
  no "BSL phase='${bph:-unset}' (no cloud-credentials secret on test). See D-WAIT-005."
fi

echo "== B-4: Kasten K10 does NOT auto-protect dr-test on $DST_CTX =="
# K10 policies live in kasten-io; confirm none selects dr-test / test-app ns.
hits=$(kubectl --context "$DST_CTX" get policies.config.kio.kasten.io -n kasten-io \
        -o yaml 2>/dev/null | grep -ciE 'dr-test' || true)
if [ "${hits:-0}" = "0" ]; then ok "no K10 policy references dr-test"; else
  no "a K10 policy references dr-test — confirm isolation before B. See D-WAIT-006."
fi

echo "----------------------------------------"
if [ "$FAIL" = "0" ]; then
  echo "PREFLIGHT B: PASS — safe to run ./run-b.sh"
  exit 0
else
  echo "PREFLIGHT B: BLOCKED — do NOT run B. Resolve the ✗ items (see 等待决策.md)."
  exit 1
fi
