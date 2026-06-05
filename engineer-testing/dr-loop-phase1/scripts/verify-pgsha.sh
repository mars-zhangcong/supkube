#!/usr/bin/env bash
# verify-pgsha.sh — cross-cluster data integrity check.
#
# Runs `pg_dump` of the testdb INSIDE each postgres pod (psql/pg_dump not
# installed locally) and sha256sums the dump, then compares SRC vs DST.
# A matching SHA proves the restored data is byte-identical to the source.
#
# We dump only table data in a deterministic way:
#   pg_dump --data-only --no-owner --no-privileges --table=dr_checkpoint
# (schema/owner noise excluded so the SHA reflects ROW DATA, not catalog
# OIDs that legitimately differ across clusters). sha256sum runs in-pod too
# so we never depend on a local postgres client.
#
# Contexts pinned via $SRC_CTX / $DST_CTX.
#
# Usage:
#   SRC_CTX=aks-jumborca-dev DST_CTX=aks-jumborca-test ./verify-pgsha.sh
set -euo pipefail

SRC_CTX="${SRC_CTX:?set SRC_CTX (source kube-context)}"
DST_CTX="${DST_CTX:?set DST_CTX (destination kube-context)}"
NS="${NS:-dr-test}"
POD="${POD:-postgres-0}"
TABLE="${TABLE:-dr_checkpoint}"

dump_sha() {
  local ctx="$1"
  kubectl --context "$ctx" -n "$NS" exec "$POD" -- sh -c \
    "pg_dump -U testuser -d testdb --data-only --no-owner --no-privileges --table=${TABLE} | sha256sum | cut -d' ' -f1"
}

echo "[verify-pgsha] dumping SRC ($SRC_CTX)..."
SRC_SHA="$(dump_sha "$SRC_CTX" | tr -d '[:space:]')"
echo "[verify-pgsha] SRC sha256 = $SRC_SHA"

echo "[verify-pgsha] dumping DST ($DST_CTX)..."
DST_SHA="$(dump_sha "$DST_CTX" | tr -d '[:space:]')"
echo "[verify-pgsha] DST sha256 = $DST_SHA"

if [[ "$SRC_SHA" == "$DST_SHA" && -n "$SRC_SHA" ]]; then
  echo "[verify-pgsha] PASS — data identical across clusters"
  exit 0
else
  echo "[verify-pgsha] FAIL — SHA mismatch (SRC=$SRC_SHA DST=$DST_SHA)" >&2
  exit 1
fi
