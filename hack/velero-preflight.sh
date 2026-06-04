#!/usr/bin/env bash
# velero-preflight.sh — fast, read-only health gate for *our own* Velero.
#
# Why this exists (2026-06-04 RCA): test cluster's Velero sat in Init:0/2 for
# 38h and nobody knew — our own DR was broken and silent. Root cause was a
# trivially detectable one: the `cloud-credentials` secret the Velero pod
# mounts did not exist in Velero's namespace (it had been created in the wrong
# namespace). A pod that can't mount a volume never even starts its init
# containers, so `kubectl get pod` shows the misleading `Init:0/2` and the BSL
# never goes Available.
#
# verify-cluster.sh checks Velero too, but (a) it assumes ns=velero (test runs
# Velero in `supkube`), and (b) it doesn't check the credential SECRET the pod
# depends on — so it reported "Velero not installed in 'velero'" and missed the
# real failure. This script is the focused complement:
#   - auto-discovers the namespace Velero actually runs in
#   - checks the POD is Running (not just that a Deployment object exists)
#   - checks every secret the Velero pod / node-agent MOUNTS actually exists
#     IN THAT namespace, and flags the "secret lives in another ns" anti-pattern
#   - checks the BSL is Available
#
# Run it:
#   - per context:   ./velero-preflight.sh aks-jumborca-test
#   - all contexts:  ./velero-preflight.sh --all
#   - periodically (the real fix for "nobody knew for 38h"): cron / scheduled
#     agent every 15-30 min, alert on non-zero exit. See RUNBOOK §1.5.
#
# READ-ONLY. Never mutates the cluster. Exit codes:
#   0 = Velero healthy (pod Running, all mounted secrets present, BSL Available)
#   1 = broken (one or more hard checks failed)
#   2 = Velero not found in this context (may be intentional; see ADR-023)
set -uo pipefail

if [[ -t 1 ]]; then
  C_OK="\033[32m"; C_WARN="\033[33m"; C_ERR="\033[31m"; C_DIM="\033[2m"; C_OFF="\033[0m"
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_OFF=""
fi
ok()   { echo -e "  ${C_OK}✓${C_OFF} $1"; }
warn() { echo -e "  ${C_WARN}!${C_OFF} $1"; }
bad()  { echo -e "  ${C_ERR}✗${C_OFF} $1"; }
dim()  { echo -e "  ${C_DIM}·${C_OFF} $1"; }

TIMEOUT="${KUBECTL_TIMEOUT:-20s}"

# check_context CTX -> sets global RC (0 ok / 1 broken / 2 not-found)
check_context() {
  local ctx="$1"
  local k=(kubectl --request-timeout="$TIMEOUT")
  [[ -n "$ctx" ]] && k+=(--context "$ctx")
  echo ""
  echo -e "${C_OK}══${C_OFF} context: ${ctx:-<current>}"

  if ! "${k[@]}" get ns >/dev/null 2>&1; then
    bad "cannot reach cluster API (context unreachable / creds expired)"
    RC=1; return
  fi

  # 1) Find EVERY namespace a Velero Deployment runs in. We check all of them,
  #    not just the first: clusters in the wild end up with Velero in more than
  #    one ns (dev had it in both `velero` and `supkube`), and that ambiguity
  #    is itself worth surfacing.
  local nslist
  nslist=$("${k[@]}" get deploy -A -l app.kubernetes.io/name=velero \
            -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null \
            | sort -u | grep -v '^$')
  if [[ -z "$nslist" ]]; then
    nslist=$("${k[@]}" get deploy -A --no-headers 2>/dev/null \
              | awk '$2=="velero"{print $1}' | sort -u)
  fi
  if [[ -z "$nslist" ]]; then
    warn "no Velero Deployment found in any namespace (intentional? see ADR-023)"
    RC=2; return
  fi

  # SupKube's DR controllers (fingerprint runner + BSL/secret reader) HARDCODE
  # the `velero` namespace (backend internal/fingerprint, internal/importpolicy).
  # If Velero only runs elsewhere, DR features won't see it even if Velero is
  # otherwise healthy — flag it.
  if ! grep -qx "velero" <<< "$nslist"; then
    warn "Velero runs in [$(paste -sd, - <<< "$nslist")] but NOT in 'velero' ns — SupKube DR (fingerprint/import) hardcodes the 'velero' ns and will not use it"
  fi

  local hard=0 ns
  while IFS= read -r ns; do
    [[ -z "$ns" ]] && continue
    dim "── Velero in ns/$ns ──"

    # 2) Velero SERVER pod must be Running. Use `name=velero` (the server) — the
    #    broader app.kubernetes.io/name=velero label also matches node-agent.
    local pod phase
    pod=$("${k[@]}" -n "$ns" get pod -l name=velero \
          -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    [[ -z "$pod" ]] && pod=$("${k[@]}" -n "$ns" get pod \
          -l app.kubernetes.io/name=velero,component=velero \
          -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [[ -z "$pod" ]]; then
      bad "no Velero server pod found in ns/$ns"; hard=1
    else
      phase=$("${k[@]}" -n "$ns" get pod "$pod" -o jsonpath='{.status.phase}' 2>/dev/null)
      if [[ "$phase" == "Running" ]]; then
        ok "velero pod $pod Running"
      else
        bad "velero pod $pod is '$phase' (Init:0/2 usually = a mounted secret is missing — see next check)"
        hard=1
      fi
    fi

    # 3) ROOT-CAUSE CHECK: every secret the Velero pod + node-agent MOUNT must
    #    exist IN THIS namespace. A missing mounted secret => pod stuck before
    #    init containers ever run. Also flag the "secret is in the wrong ns".
    local wf
    for wf in "deploy/velero" "ds/node-agent"; do
      "${k[@]}" -n "$ns" get "$wf" >/dev/null 2>&1 || continue
      local secs
      secs=$("${k[@]}" -n "$ns" get "$wf" \
              -o jsonpath='{range .spec.template.spec.volumes[*]}{.secret.secretName}{"\n"}{end}' \
              2>/dev/null | sort -u | grep -v '^$')
      [[ -z "$secs" ]] && continue
      local s
      while IFS= read -r s; do
        [[ -z "$s" ]] && continue
        if "${k[@]}" -n "$ns" get secret "$s" >/dev/null 2>&1; then
          ok "$wf mounts secret '$s' — present in ns/$ns"
        else
          # Is it sitting in a DIFFERENT namespace? (the exact 2026-06-04 bug)
          local elsewhere
          elsewhere=$("${k[@]}" get secret -A --field-selector "metadata.name=$s" \
                       --no-headers 2>/dev/null | awk '{print $1}' | paste -sd, -)
          if [[ -n "$elsewhere" ]]; then
            bad "$wf mounts secret '$s' — MISSING in ns/$ns but EXISTS in: $elsewhere  (wrong-namespace placement — copy it into ns/$ns)"
          else
            bad "$wf mounts secret '$s' — MISSING everywhere (create the cloud-credentials secret in ns/$ns)"
          fi
          hard=1
        fi
      done <<< "$secs"
    done

    # 4) BSL must be Available.
    local rows
    rows=$("${k[@]}" -n "$ns" get bsl \
            -o jsonpath='{range .items[*]}{.metadata.name}={.status.phase}{"\n"}{end}' 2>/dev/null \
            | grep -v '^$')
    if [[ -z "$rows" ]]; then
      warn "no BackupStorageLocation in ns/$ns (Velero here can't store backups)"
    else
      local r name ph
      while IFS= read -r r; do
        name="${r%%=*}"; ph="${r#*=}"
        if [[ "$ph" == "Available" ]]; then
          ok "BSL $name Available"
        else
          bad "BSL $name phase='${ph:-empty}' (empty usually = velero server never started, i.e. check 2/3 failed)"
          hard=1
        fi
      done <<< "$rows"
    fi
  done <<< "$nslist"

  if [[ "$hard" -eq 0 ]]; then RC=0; else RC=1; fi
}

# ── main ───────────────────────────────────────────────────────────
OVERALL=0
if [[ "${1:-}" == "--all" ]]; then
  while IFS= read -r ctx; do
    RC=0; check_context "$ctx"
    [[ "$RC" -eq 1 ]] && OVERALL=1
  done < <(kubectl config get-contexts -o name 2>/dev/null)
else
  RC=0; check_context "${1:-}"
  OVERALL="$RC"
fi

echo ""
if [[ "$OVERALL" -eq 0 ]]; then
  echo -e "${C_OK}══ velero-preflight: OK${C_OFF}"
elif [[ "$OVERALL" -eq 2 ]]; then
  echo -e "${C_WARN}══ velero-preflight: Velero not found (see ADR-023)${C_OFF}"
else
  echo -e "${C_ERR}══ velero-preflight: BROKEN — our own DR is not healthy. See RUNBOOK §1.5.${C_OFF}"
fi
exit "$OVERALL"
