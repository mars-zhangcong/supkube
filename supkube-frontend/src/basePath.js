// Runtime base path. nginx entrypoint writes window.__SUPKUBE_CONFIG__={basePath:"..."}
// into /config.js (loaded before main.js). Defaults to '/' for dev + the
// zero-regression default deployment.
const raw = (typeof window !== 'undefined' && window.__SUPKUBE_CONFIG__ && window.__SUPKUBE_CONFIG__.basePath) || '/'
// router base: leading slash, single, no trailing (except root '/')
export const basePath = raw === '/' || raw === '' ? '/' : '/' + raw.replace(/^\/+|\/+$/g, '')
// prefix for building absolute URLs: '' at root, '/supkube' otherwise
export const basePrefix = basePath === '/' ? '' : basePath
// NOTE: do NOT export a module-level `apiBase = basePrefix + '/api/v1'` — when
// consumed at module-init (axios baseURL) esbuild const-folds basePrefix to ""
// and bakes "/api/v1", dropping the subpath prefix. Build the API URL at
// request time from basePrefix (see api/velero.js). loginPath/callbackPath are
// safe because they're only read inside function bodies.
export const loginPath = basePrefix + '/login'
export const callbackPath = basePrefix + '/auth/callback'
