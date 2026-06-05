#!/usr/bin/env bash
# verify-count.sh — assert dr_checkpoint row count == expected N.
#
# psql is NOT installed locally, so the SELECT runs via `kubectl exec` into
# the postgres-0 pod. Context is pinned via $CTX (defaults to $SRC_CTX).
#
# Usage:
#   CTX=aks-jumborca-dev  ./verify-count.sh 5
#   SRC_CTX=aks-jumborca-dev ./verify-count.sh 5
#   DST_CTX=aks-jumborca-test CTX=aks-jumborca-test ./verify-count.sh 5
set -euo pipefail

EXPECTED="${1:?usage: verify-count.sh <expected-row-count> (e.g. 5)}"
CTX="${CTX:-${SRC_CTX:-}}"
NS="${NS:-dr-test}"
POD="${POD:-postgres-0}"

if [[ -z "$CTX" ]]; then
  echo "ERROR: set CTX (or SRC_CTX/DST_CTX) to the kube-context to check" >&2
  exit 2
fi

echo "[verify-count] context=$CTX ns=$NS pod=$POD expected=$EXPECTED"

ACTUAL="$(kubectl --context "$CTX" -n "$NS" exec "$POD" -- \
  psql -U testuser -d testdb -tAc 'SELECT count(*) FROM dr_checkpoint;' \
  | tr -d '[:space:]')"

echo "[verify-count] actual row count = $ACTUAL"

if [[ "$ACTUAL" == "$EXPECTED" ]]; then
  echo "[verify-count] PASS"
  exit 0
else
  echo "[verify-count] FAIL: expected $EXPECTED, got $ACTUAL" >&2
  exit 1
fi
