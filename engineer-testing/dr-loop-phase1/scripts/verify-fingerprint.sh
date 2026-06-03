#!/usr/bin/env bash
# verify-fingerprint.sh — read .supkube-fingerprint.json from the BSL and
# print the fields the DR loop cares about for a forward/reverse match:
#   tarballSHA256  (the brief's "rp_sha256" — actual field name is
#                   tarballSHA256 per internal/fingerprint/types.go)
#   sourceClusterID
#   backupName / createdAt / hmacSHA256 (for the audit trail)
#
# Blob path layout (internal/fingerprint/types.go):
#   <bsl-prefix>/backups/<backup-name>/.supkube-fingerprint.json
#
# AUTHZ NOTE: reading this blob from Azure requires the caller identity to
# hold the **Storage Blob Data Reader** role on the storage account/container
# (plain "Reader"/control-plane RBAC is NOT enough to read blob bytes).
#
# Two fetch backends:
#   BACKEND=az   (default) — az storage blob download, needs the role above.
#   BACKEND=api            — GET SupKube API /imports/preview style endpoint
#                            (set SUPKUBE_API + auth via $SUPKUBE_TOKEN).
#
# Usage (az):
#   AZ_ACCOUNT=mystorage AZ_CONTAINER=velero BSL_PREFIX=dev \
#     ./verify-fingerprint.sh test-dr-loop-001
set -euo pipefail

BACKUP_NAME="${1:?usage: verify-fingerprint.sh <backup-name>}"
BACKEND="${BACKEND:-az}"
FP_FILE=".supkube-fingerprint.json"

emit() {
  # $1 = raw JSON of the fingerprint document
  local json="$1"
  if command -v jq >/dev/null 2>&1; then
    echo "$json" | jq -r '
      "backupName      = \(.backupName)",
      "sourceClusterID = \(.sourceClusterID)",
      "tarballSHA256   = \(.tarballSHA256)   # brief calls this rp_sha256",
      "createdAt       = \(.createdAt)",
      "hmacSHA256      = \(.hmacSHA256)",
      "algo            = \(.algo)"'
  else
    echo "[verify-fingerprint] jq not found; raw JSON:" >&2
    echo "$json"
  fi
}

case "$BACKEND" in
  az)
    AZ_ACCOUNT="${AZ_ACCOUNT:?set AZ_ACCOUNT (storage account name)}"
    AZ_CONTAINER="${AZ_CONTAINER:?set AZ_CONTAINER (blob container, usually 'velero')}"
    BSL_PREFIX="${BSL_PREFIX:-}"   # e.g. "dev" if BSL has a prefix; empty = root
    if [[ -n "$BSL_PREFIX" ]]; then
      BLOB="${BSL_PREFIX}/backups/${BACKUP_NAME}/${FP_FILE}"
    else
      BLOB="backups/${BACKUP_NAME}/${FP_FILE}"
    fi
    echo "[verify-fingerprint] az download az://${AZ_ACCOUNT}/${AZ_CONTAINER}/${BLOB}" >&2
    echo "[verify-fingerprint] NOTE: requires 'Storage Blob Data Reader' on the account" >&2
    JSON="$(az storage blob download \
      --account-name "$AZ_ACCOUNT" \
      --container-name "$AZ_CONTAINER" \
      --name "$BLOB" \
      --auth-mode login \
      --no-progress -o tsv --query content 2>/dev/null)"
    ;;
  api)
    SUPKUBE_API="${SUPKUBE_API:?set SUPKUBE_API (e.g. http://localhost:8080)}"
    AUTH=()
    [[ -n "${SUPKUBE_TOKEN:-}" ]] && AUTH=(-H "Authorization: Bearer ${SUPKUBE_TOKEN}")
    echo "[verify-fingerprint] GET ${SUPKUBE_API}/api/v1/imports/fingerprint?backup=${BACKUP_NAME}" >&2
    JSON="$(curl -fsS "${AUTH[@]}" \
      "${SUPKUBE_API}/api/v1/imports/fingerprint?backup=${BACKUP_NAME}")"
    ;;
  *)
    echo "ERROR: unknown BACKEND=$BACKEND (use az|api)" >&2
    exit 2
    ;;
esac

if [[ -z "${JSON:-}" ]]; then
  echo "[verify-fingerprint] FAIL — could not read $FP_FILE for backup '$BACKUP_NAME'" >&2
  echo "  (check the backup name, BSL prefix, and Storage Blob Data Reader role)" >&2
  exit 1
fi

emit "$JSON"
echo "[verify-fingerprint] OK — fingerprint present and parsed"
