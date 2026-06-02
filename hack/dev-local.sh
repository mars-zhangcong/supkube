#!/usr/bin/env bash
# dev-local.sh — SupKube fast local inner-loop for Vibe Coding.
#
# WHY THIS EXISTS
# ───────────────
# The "real" deploy path is slow on purpose: edit → docker build (multi-arch,
# minutes) → push to ACR → CI/CD → pull into a remote AKS cluster → pod
# restart. That's the right loop for SHIPPING, but a terrible loop for
# ITERATING — especially on UI, where you may want 30 tiny tweaks in a row.
#
# This script collapses that loop back to seconds by running the pieces the
# way they were designed to run during development:
#
#   • Frontend (Vue 3 + Vite) already has Hot Module Replacement and already
#     proxies `/api` → http://localhost:8080 (see supkube-frontend/vite.config.js).
#     So a saved .vue file shows up in the browser in <1s, no image, no cluster.
#
#   • The backend just needs *a* Kubernetes API to talk to. Two ways to give it
#     one without rebuilding anything — pick with --mode:
#
# MODES
# ─────
#   --mode full      (default) Run the Go backend locally with `go run`
#                    (~seconds to start, auth OFF) AND the Vite dev server.
#                    The backend talks to your CURRENT kube-context's API
#                    server (remote is fine — client-go calls it over the
#                    network). Use this when you're changing backend code too.
#
#   --mode ui        UI-ONLY. Don't run Go at all. Port-forward the backend
#                    that's ALREADY deployed in the cluster to localhost:8080,
#                    and run only the Vite dev server against it. This is the
#                    fastest possible loop for pure frontend / "change a bunch
#                    of UI things" work — real backend data, zero Go build.
#
# In BOTH modes the browser URL is the same:  http://localhost:3000
#
# USAGE
# ─────
#   ./hack/dev-local.sh                       # full stack, current context
#   ./hack/dev-local.sh --mode ui             # UI-only, port-forward cluster backend
#   ./hack/dev-local.sh --context docker-desktop
#   ./hack/dev-local.sh --mode ui --context aks-jumborca-dev --namespace supkube
#
# Press Ctrl-C once to stop everything (backend / port-forward / vite are all
# cleaned up together).
#
# FLAGS
# ─────
#   --mode full|ui      Which loop to run (default: full)
#   --context NAME      kube-context to use (default: current)
#   --namespace NS      namespace of the deployed backend, ui mode (default: supkube)
#   --backend-port N    local port for the backend / port-forward (default: 8080)
#   --frontend-port N   local port for the Vite dev server (default: 3000)
#   -h, --help          show this help

set -euo pipefail

# ── locate repo root from this script's location (works from anywhere) ──────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/supkube-backend"
FRONTEND_DIR="$REPO_ROOT/supkube-frontend"

# ── defaults ────────────────────────────────────────────────────────────────
MODE="full"
KCTX=""
NAMESPACE="supkube"
BACKEND_PORT="8080"
FRONTEND_PORT="3000"
BACKEND_SVC="supkube-backend"

# ── colours (no-op if not a tty) ─────────────────────────────────────────────
if [[ -t 1 ]]; then
  B="\033[1m"; G="\033[32m"; Y="\033[33m"; C="\033[36m"; R="\033[31m"; N="\033[0m"
else
  B=""; G=""; Y=""; C=""; R=""; N=""
fi
say()  { echo -e "${C}>${N} $*"; }
ok()   { echo -e "${G}OK${N} $*"; }
warn() { echo -e "${Y}!${N} $*"; }
die()  { echo -e "${R}ERROR${N} $*" >&2; exit 1; }

# ── parse flags ──────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode)          MODE="${2:-}"; shift 2 ;;
    --context)       KCTX="${2:-}"; shift 2 ;;
    --namespace)     NAMESPACE="${2:-}"; shift 2 ;;
    --backend-port)  BACKEND_PORT="${2:-}"; shift 2 ;;
    --frontend-port) FRONTEND_PORT="${2:-}"; shift 2 ;;
    -h|--help)       sed -n '2,/^set -euo/p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//; s/^#//' | sed '$d'; exit 0 ;;
    *)               die "unknown flag: $1 (try --help)" ;;
  esac
done
[[ "$MODE" == "full" || "$MODE" == "ui" ]] || die "--mode must be 'full' or 'ui' (got '$MODE')"

# ── prerequisite checks ──────────────────────────────────────────────────────
need() { command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"; }
need node
need npm
need kubectl
[[ "$MODE" == "full" ]] && need go

CTX_FLAG=()
if [[ -n "$KCTX" ]]; then CTX_FLAG=(--context "$KCTX"); fi
CUR_CTX="$(kubectl config current-context "${CTX_FLAG[@]}" 2>/dev/null || true)"
[[ -n "$CUR_CTX" ]] || die "no kube-context available (set one with --context or 'kubectl config use-context')"

# ── child-process bookkeeping + cleanup on exit ──────────────────────────────
PIDS=()
cleanup() {
  echo
  say "shutting down…"
  for pid in "${PIDS[@]:-}"; do
    [[ -n "${pid:-}" ]] && kill "$pid" 2>/dev/null || true
  done
  # give children a moment, then hard-kill any stragglers
  sleep 1
  for pid in "${PIDS[@]:-}"; do
    [[ -n "${pid:-}" ]] && kill -9 "$pid" 2>/dev/null || true
  done
  ok "done."
}
trap cleanup EXIT INT TERM

# ── ensure frontend deps are present (first run only) ────────────────────────
if [[ ! -d "$FRONTEND_DIR/node_modules" ]]; then
  say "installing frontend deps (first run)…"
  (cd "$FRONTEND_DIR" && npm install)
fi

echo
echo -e "${B}SupKube dev-local${N}  —  mode=${B}$MODE${N}  context=${B}$CUR_CTX${N}"
echo "────────────────────────────────────────────────────────────"

# ── start the backend side ───────────────────────────────────────────────────
if [[ "$MODE" == "full" ]]; then
  say "starting backend (go run, auth OFF) on :$BACKEND_PORT …"
  # client-go's getConfig() falls back to a kubeconfig file, using its
  # current-context. To honour --context WITHOUT mutating the user's global
  # kubeconfig, we hand the backend a minified, flattened, single-context
  # kubeconfig via $KUBECONFIG.
  TMP_KUBECONFIG="$(mktemp -t supkube-dev-kubeconfig.XXXXXX)"
  kubectl config view --minify --flatten "${CTX_FLAG[@]}" > "$TMP_KUBECONFIG"
  trap 'rm -f "$TMP_KUBECONFIG"' EXIT  # augmented below; final trap re-set
  (
    cd "$BACKEND_DIR"
    KUBECONFIG="$TMP_KUBECONFIG" AUTH_ENABLED=false go run main.go
  ) &
  PIDS+=("$!")
  # re-establish the full cleanup trap (the rm-only trap above would have
  # clobbered it); cleanup() handles both pids and the temp file.
  trap 'cleanup; rm -f "$TMP_KUBECONFIG"' EXIT INT TERM
  warn "backend talks to the REAL API server of context '$CUR_CTX' (controllers will run)."
else
  say "port-forwarding deployed backend svc/$BACKEND_SVC -n $NAMESPACE → :$BACKEND_PORT …"
  kubectl "${CTX_FLAG[@]}" -n "$NAMESPACE" port-forward "svc/$BACKEND_SVC" \
    "$BACKEND_PORT:8080" >/dev/null 2>&1 &
  PIDS+=("$!")
  sleep 2  # let the forward establish before vite starts proxying
fi

# ── start the frontend (Vite, HMR) ───────────────────────────────────────────
say "starting frontend (Vite + HMR) on :$FRONTEND_PORT …"
echo
ok  "Open  ${B}http://localhost:$FRONTEND_PORT${N}   (edits to .vue files appear in <1s)"
echo "    API  /api → http://localhost:$BACKEND_PORT"
echo "    Stop everything with Ctrl-C"
echo "────────────────────────────────────────────────────────────"
echo

# Run vite in the FOREGROUND so its logs stream and Ctrl-C lands here,
# triggering the cleanup trap that tears down the backend/port-forward too.
(cd "$FRONTEND_DIR" && npm run dev -- --port "$FRONTEND_PORT")
