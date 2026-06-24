#!/bin/sh
# PRD-028 / ADR-056 v2 — SupKube frontend base-path entrypoint.
#
# Runs from /docker-entrypoint.d/ before nginx starts (nginx:alpine convention,
# scripts run in lexical order; 20-envsubst-on-templates.sh has already expanded
# ${BASE_PATH} in the nginx template by the time we run at 30-).
#
# Responsibility: anchor the static SPA shell at the deployed URL subpath by
#   1. normalizing BASE_PATH,
#   2. rewriting <base href="..."> in index.html,
#   3. writing /config.js so the SPA learns its basePath at runtime.
# The favicon (and every other asset) is already a RELATIVE URL in index.html,
# so it resolves against the rewritten <base href> automatically — no separate
# favicon rewrite is needed.
#
# Single image, any path. At BASE_PATH=/ (the default) this reproduces today's
# behavior byte-for-byte: <base href="/">, config.js basePath "/".
#
# IDEMPOTENT: guarded by a marker file so a container restart (nginx re-runs
# /docker-entrypoint.d on every start) does not double-rewrite index.html.
set -eu

HTML_DIR="/usr/share/nginx/html"
INDEX="${HTML_DIR}/index.html"
CONFIG_JS="${HTML_DIR}/config.js"
MARKER="${HTML_DIR}/.supkube-basepath.done"

# --- normalize BASE_PATH --------------------------------------------------
# Accept "/", "", "/supkube", "supkube", "/supkube/" → canonical prefix.
#   "/" or empty            -> PREFIX="" , CONFIG_BASE="/"
#   "/supkube" (any slashes)-> PREFIX="/supkube", CONFIG_BASE="/supkube"
RAW="${BASE_PATH:-/}"
# strip leading and trailing slashes
TRIMMED=$(printf '%s' "$RAW" | sed -e 's#^/*##' -e 's#/*$##')
if [ -z "$TRIMMED" ]; then
  PREFIX=""
  CONFIG_BASE="/"
else
  PREFIX="/${TRIMMED}"
  CONFIG_BASE="/${TRIMMED}"
fi

# --- idempotency guard ----------------------------------------------------
# Re-run only if the recorded base differs (lets a pod that somehow gets a new
# BASE_PATH without a fresh image still converge; normally never triggers).
if [ -f "$MARKER" ] && [ "$(cat "$MARKER" 2>/dev/null || true)" = "$CONFIG_BASE" ]; then
  echo "30-supkube-basepath: already applied for base '${CONFIG_BASE}', skipping"
  exit 0
fi

# --- 1. <base href> -------------------------------------------------------
# The built index.html ships with <base href="/"> (dev/default). Rewrite to
# the deployed base. <base href> must end in a trailing slash for relative
# URL resolution; "/" stays "/", "/supkube" -> "/supkube/".
if [ -z "$PREFIX" ]; then
  HREF="/"
else
  HREF="${PREFIX}/"
fi
if [ -f "$INDEX" ]; then
  # Rewrite the <base href="..."> the Vite build ships (always double-quoted).
  # Portable BRE that works on busybox (alpine), BSD and GNU sed:
  #   <base href=" ... "  ->  <base href="${HREF}"
  # We replace only the quoted value and leave the trailing ` />` (or `>`)
  # untouched. '#' delimiter avoids clashing with the slashes in $HREF.
  sed -i "s#<base href=\"[^\"]*\"#<base href=\"${HREF}\"#" "$INDEX"
fi

# --- 2. config.js ---------------------------------------------------------
# Loaded before main.js; basePath.js reads window.__SUPKUBE_CONFIG__.basePath.
printf 'window.__SUPKUBE_CONFIG__={basePath:"%s"};\n' "$CONFIG_BASE" > "$CONFIG_JS"

echo "30-supkube-basepath: base href='${HREF}', config basePath='${CONFIG_BASE}'"
printf '%s' "$CONFIG_BASE" > "$MARKER"
