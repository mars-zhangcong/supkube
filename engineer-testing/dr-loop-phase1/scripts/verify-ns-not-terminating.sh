#!/usr/bin/env bash
# verify-ns-not-terminating.sh — assert the dr-test namespace is NOT stuck
# in `Terminating` (the LoadBalancer Service trap). This is Gate 5: the
# lb-to-clusterip Transform must have converted the Service so cleanup
# tears down cleanly.
#
# Behaviour:
#   - If the namespace does NOT exist  -> PASS (cleanly deleted).
#   - If it exists with phase=Active   -> PASS (alive, not wedged).
#   - If it exists with phase=Terminating beyond GRACE seconds -> FAIL.
#
# Context pinned via $CTX (defaults to $DST_CTX then $SRC_CTX).
#
# Usage:
#   CTX=aks-jumborca-test ./verify-ns-not-terminating.sh
#   CTX=aks-jumborca-test NS=dr-test GRACE=60 ./verify-ns-not-terminating.sh
set -euo pipefail

CTX="${CTX:-${DST_CTX:-${SRC_CTX:-}}}"
NS="${NS:-dr-test}"
GRACE="${GRACE:-0}"   # seconds of Terminating tolerated before FAIL

if [[ -z "$CTX" ]]; then
  echo "ERROR: set CTX (or DST_CTX/SRC_CTX)" >&2
  exit 2
fi

echo "[verify-ns] context=$CTX ns=$NS grace=${GRACE}s"

PHASE="$(kubectl --context "$CTX" get ns "$NS" -o jsonpath='{.status.phase}' 2>/dev/null || true)"

if [[ -z "$PHASE" ]]; then
  echo "[verify-ns] namespace '$NS' not found -> PASS (cleanly gone)"
  exit 0
fi

echo "[verify-ns] namespace '$NS' phase = $PHASE"

if [[ "$PHASE" == "Terminating" ]]; then
  if [[ "$GRACE" -gt 0 ]]; then
    echo "[verify-ns] Terminating; waiting up to ${GRACE}s for it to clear..."
    for ((i=0; i<GRACE; i++)); do
      sleep 1
      P="$(kubectl --context "$CTX" get ns "$NS" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
      [[ -z "$P" ]] && { echo "[verify-ns] cleared -> PASS"; exit 0; }
    done
  fi
  echo "[verify-ns] FAIL — namespace stuck in Terminating" >&2
  echo "[verify-ns] dangling finalizers / LB Service likely; inspect:" >&2
  echo "  kubectl --context $CTX get svc -n $NS" >&2
  echo "  kubectl --context $CTX get ns $NS -o jsonpath='{.spec.finalizers}'" >&2
  exit 1
fi

echo "[verify-ns] PASS (phase=$PHASE, not Terminating)"
exit 0
