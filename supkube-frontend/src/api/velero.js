import axios from 'axios'

// Always use a relative base URL so requests go through the same-origin nginx
// reverse proxy (`/api/` → supkube-backend:8080). The previous default of
// `http://localhost:8080` was wrong in production — it tried to cross-origin
// hit a port that isn't exposed by the K8S Service, and worked only when the
// user happened to have a `kubectl port-forward 8080:8080` running, otherwise
// silently fell back to stale HTTP cache. If you need to override for local
// dev outside K8S, set VITE_API_URL=http://localhost:8080/api/v1.
const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'

const api = axios.create({ baseURL: BASE_URL })

// Per-request cache-buster: append `_=<ts>` to every GET URL so the browser
// treats it as a unique resource and never serves a cached prior response.
// We deliberately do NOT set Cache-Control/Pragma request headers — those
// would trigger CORS preflight, and they're redundant with the cache-buster
// param plus the server-side `Cache-Control: no-store` we send on responses.
api.interceptors.request.use((config) => {
  if ((config.method || 'get').toLowerCase() === 'get') {
    config.params = { ...(config.params || {}), _: Date.now() }
  }
  // v0.8.5: attach Bearer token to every request when available.
  // Stored by useAuth.handleCallback() after the OIDC redirect dance.
  const token = localStorage.getItem('supkube.idToken')
  if (token) {
    config.headers = config.headers || {}
    config.headers.Authorization = `Bearer ${token}`
  }
  // v0.9.0 MC1: attach X-Supkube-Cluster header when a remote cluster is
  // selected. Empty / 'this-cluster' / '_mcm' all skip the header so the
  // backend routes against its own kubeconfig (existing single-cluster
  // path). The composable's storage key is the canonical source.
  const active = localStorage.getItem('supkube.activeCluster')
  if (active && active !== 'this-cluster' && active !== '_mcm') {
    config.headers = config.headers || {}
    config.headers['X-Supkube-Cluster'] = active
  }
  return config
})

// v0.8.5: 401 interceptor. When the backend says "your token's no good",
// drop the user back on the login screen — likely the token expired or
// auth was just turned on. We intentionally do NOT auto-refresh here;
// refresh logic stays in useAuth so it can show UX (loading state etc).
//
// v0.8.11.2 fix: skip the redirect on /auth/callback. Otherwise the
// eager fetchBranding() in useBranding.js fires a 401 the moment the
// SPA boots on the OIDC return URL, which pre-empts handleCallback()
// and cancels the in-flight POST /auth/callback (logs show 499). The
// callback page MUST be allowed to finish the code-exchange even
// when other requests return 401 — that's literally the moment we go
// from "no token" to "valid token", so 401s during this window are
// expected, not a session-expiry signal.
api.interceptors.response.use(
  r => r,
  (err) => {
    if (err?.response?.status === 401) {
      const path = window.location.pathname
      // Avoid redirect loops if we're already on /login or processing
      // the OIDC callback (code-exchange is in flight there).
      if (path !== '/login' && path !== '/auth/callback') {
        localStorage.removeItem('supkube.idToken')
        localStorage.removeItem('supkube.refreshToken')
        localStorage.removeItem('supkube.user')
        window.location.href = '/login?reason=expired'
      }
    }
    return Promise.reject(err)
  }
)

// v0.8.5 Auth
export const getAuthProviders = () => api.get('/auth/providers')
export const authCallback = (data) => api.post('/auth/callback', data)
export const getAuthMe = () => api.get('/auth/me')
export const authLogout = () => api.post('/auth/logout')
// v0.8.5 step 3.5: read-only RBAC bindings (admin-only)
export const getRBACBindings = () => api.get('/auth/rbac/bindings')
// v0.8.5 step 4: audit log query (admin-only)
export const getAuditLogs = (params = {}) => api.get('/audit-logs', { params })

export const getStatus = () => api.get('/status')
export const getDashboardSummary = () => api.get('/dashboard/summary')
export const getNamespaces = () => api.get('/namespaces')
export const createNamespace = (name) => api.post('/namespaces', { name })
export const getApplications = () => api.get('/applications')
export const getApplicationDetail = (namespace) => api.get(`/applications/${namespace}/details`)
export const getNamespaceStorageCapability = (namespace) => api.get(`/applications/${namespace}/storage-capability`)

// Backup Advisor (v0.7.5)
export const getBackupAdvisor = () => api.get('/backup-advisor')
export const getBackupAdvisorForNamespace = (namespace) => api.get(`/backup-advisor/${namespace}`)

// Backups
export const getBackups = () => api.get('/backups')
export const getBackup = (name) => api.get(`/backups/${name}`)
export const getBackupResources = (name) => api.get(`/backups/${name}/resources`)
export const getBackupArtifacts = (name) => api.get(`/backups/${name}/artifacts`)
export const getBackupLogs = (name) => api.get(`/backups/${name}/logs`)
export const createBackup = (data) => api.post('/backups', data)
export const deleteBackup = (name) => api.delete(`/backups/${name}`)
// v0.9.x #68: emergency escape — strips finalizers + direct delete (no
// DBR cascade). Only call from confirm dialog warning the user about
// bypassed side effects. Backend doc: ForceDeleteBackup in handlers.go.
export const forceDeleteBackup = (name) => api.post(`/backups/${name}/force-delete`)

// v0.9.x #79 Log Viewer. Two GETs; download returns a blob so the
// browser save dialog fires. Bearer token is attached by the shared
// axios interceptor, so blob download works under OIDC auth.
export const getLogComponents = () => api.get('/logs/components')
export const getLogs          = (params) => api.get('/logs', { params })
export const downloadLogs     = (params) => api.get('/logs', { params: { ...params, download: 1 }, responseType: 'blob' })

// v0.8.0: Unified Action stream — Activity page consumes /api/v1/actions
// (Backups + Restores aggregated into a single Action shape). The Restores
// page is being retired; UI links route to /activity?type=Restore instead.
export const getActions = (params = {}) => api.get('/actions', { params })
export const getAction = (id, type) => api.get(`/actions/${encodeURIComponent(id)}`, { params: { type } })

// PRD-002 v1.3 (2026-05-31) two-layer schema:
//   - Transform: atomic rule bundle (Velero ResourceModifier rules.yaml)
//   - TransformSet: container that REFERENCES Transforms by name + sets
//     defaults for ${VAR} placeholders. No inline rules anymore.
// The on-disk ConfigMaps moved from velero ns → supkube ns; the migration
// runs server-side on startup.
export const getTransforms   = () => api.get('/transforms')
export const getTransform    = (name) => api.get(`/transforms/${name}`)
export const createTransform = (data) => api.post('/transforms', data)
export const updateTransform = (name, data) => api.put(`/transforms/${name}`, data)
// 409 when any TransformSet still references this Transform; response body
// carries `referencingTransformSets: [name, ...]` so the UI can list them.
export const deleteTransform = (name) => api.delete(`/transforms/${name}`)

export const getTransformSets   = () => api.get('/transform-sets')
export const getTransformSet    = (name) => api.get(`/transform-sets/${name}`)
export const createTransformSet = (data) => api.post('/transform-sets', data)
export const updateTransformSet = (name, data) => api.put(`/transform-sets/${name}`, data)
export const deleteTransformSet = (name) => api.delete(`/transform-sets/${name}`)
// Apply Pre-flight "Suggested Fixes" as an atomic Transform.
// Body: { restoreName, fixes: [ConflictFix, ...] }
// Response: { transformName, created, ruleCount }
//   — transformName is the auto-generated atomic Transform; the UI then
//     navigates the user to /transforms?name=<transformName> so they can
//     review and wire it into a TransformSet before triggering Restore.
export const applyConflictFixes = (data) => api.post('/transform-sets/apply-conflict-fixes', data)

// Restores (legacy — kept for backward compat during v0.8 transition)
export const getRestores = () => api.get('/restores')
export const getRestore = (name) => api.get(`/restores/${name}`)
export const createRestore = (data) => api.post('/restores', data)
// v0.7.12: Pre-flight conflict detection before restore. Body: { backupName,
// targetNamespace, cleanupBeforeRestore, deepCheck }. Returns conflicts[]
// with per-item suggestedTransform.
export const preflightRestore = (data) => api.post('/restores/preflight', data)
export const deleteRestore = (name) => api.delete(`/restores/${name}`)
export const getRestoreResults = (name) => api.get(`/restores/${name}/results`)

// Schedules
export const getSchedules = () => api.get('/schedules')
export const createSchedule = (data) => api.post('/schedules', data)
export const patchSchedule = (name, data) => api.patch(`/schedules/${name}`, data)
export const runScheduleOnce = (name) => api.post(`/schedules/${name}/run-once`)
export const getSchedule = (name) => api.get(`/schedules/${name}`)
export const deleteSchedule = (name) => api.delete(`/schedules/${name}`)

// Import Policies — 跨集群 DR：从共享 BSL 拉取另一个集群产生的 RP
// (Agent D / PRD Import Policy 2026-06-01). 共享 contract:
//   POST   /import-policies               body: {name, spec:{...}}
//   GET    /import-policies               list
//   POST   /import-policies/:n/pause
//   POST   /import-policies/:n/resume
//   POST   /import-policies/:n/run-once
export const getImportPolicies     = () => api.get('/import-policies')
export const createImportPolicy    = (data) => api.post('/import-policies', data)
export const pauseImportPolicy     = (name) => api.post(`/import-policies/${name}/pause`)
export const resumeImportPolicy    = (name) => api.post(`/import-policies/${name}/resume`)
export const runImportPolicyOnce   = (name) => api.post(`/import-policies/${name}/run-once`)
export const deleteImportPolicy    = (name) => api.delete(`/import-policies/${name}`)

// Storage locations
export const getStorageLocations = () => api.get('/storage-locations')
export const getStorageLocation = (name) => api.get(`/storage-locations/${name}`)
export const createStorageLocation = (data) => api.post('/storage-locations', data)
export const updateStorageLocation = (name, data) => api.put(`/storage-locations/${name}`, data)
export const deleteStorageLocation = (name) => api.delete(`/storage-locations/${name}`)
export const verifyStorageLocation = (name) => api.post(`/storage-locations/${name}/verify`)

// Volume snapshot locations (CSI / cloud-native snapshots)
export const getVolumeSnapshotLocations = () => api.get('/volume-snapshot-locations')
export const getVolumeSnapshotLocation = (name) => api.get(`/volume-snapshot-locations/${name}`)
export const createVolumeSnapshotLocation = (data) => api.post('/volume-snapshot-locations', data)
export const deleteVolumeSnapshotLocation = (name) => api.delete(`/volume-snapshot-locations/${name}`)

// v0.8.8 Cluster Hygiene — orphan resource cleanup
export const getCleanupSettings    = () => api.get('/settings/cleanup')
export const updateCleanupSettings = (data) => api.put('/settings/cleanup', data)
export const runOrphanCleanup      = () => api.post('/admin/cleanup/orphans')

// v0.8.8.1 Backup error details — fetched on Action Details drawer open
// when a Backup is in PartiallyFailed or Failed state. Closes the
// §11.3 silent-error gap from v0.8.3 (Restore had this since v0.7.11).
export const getBackupErrors = (name) => api.get(`/backups/${name}/errors`)

// v0.8.10.1: Kasten-style Artifacts breakdown for the Action Details drawer.
// Returns grouped item lists (Workloads / Configuration / Networking / Storage /
// RBAC / Autoscaling / CSI Snapshot CRs / Events) split into "application" vs
// "infrastructure" categories so the drawer can show "Application Items: 25"
// alongside the raw Velero progress count. See backend artifact_breakdown.go.
export const getBackupArtifactBreakdown = (name) => api.get(`/backups/${name}/artifact-breakdown`)

// v0.8.10.3: lazy YAML fetch for the per-artifact </> button in
// Action/Application/Restore Point drawers. params = { kind, name, namespace }.
export const getResourceYaml = (params) => api.get('/resources/yaml', { params })

// v0.8.11: white-label branding. GET on every app boot to render the
// sidebar + window title; PUT from Settings → Branding (admin only).
// data shape: { productName, logoDataUrl, faviconDataUrl }
export const getBranding    = () => api.get('/settings/branding')
export const updateBranding = (data) => api.put('/settings/branding', data)

// v0.8.12 LBS1: in-cluster MinIO local backup store status (read-only).
// Returns { enabled, phase, bucket, endpoint, objectLockEnabled, ... }.
// When enabled=false the rest is zero — UI shows "Not Enabled" empty state.
export const getLocalStoreStatus = () => api.get('/local-store/status')

// v0.8.12.5: DR Topology aggregator for the Dashboard's hero card.
// Returns { clusters, bsls, flows, score, summary } — one round-trip.
export const getTopology = () => api.get('/dashboard/topology')

// v0.8.13 HC4: Settings → Plugins. Returns { items: PluginStatus[] }
// with installed flag + helm upgrade command per optional subchart.
export const getPluginsStatus = () => api.get('/plugins/status')

// v0.9.0 MC1: Multi-Cluster Manager registry.
// list returns { items: ClusterDTO[] } where the SPA-facing DTO is the
// flat shape defined in backend internal/api/v1/clusters.go.
export const listClusters       = () => api.get('/clusters')
export const getCluster         = (name) => api.get(`/clusters/${encodeURIComponent(name)}`)
export const createCluster      = (data) => api.post('/clusters', data)
export const deleteCluster      = (name) => api.delete(`/clusters/${encodeURIComponent(name)}`)
// testClusterByKubeconfig: pre-submit ping in the Add Cluster wizard.
// testClusterByName: post-create re-test from the Clusters list.
export const testClusterByKubeconfig = (data) => api.post('/clusters/test', data)
export const testClusterByName       = (name) => api.post(`/clusters/${encodeURIComponent(name)}/test`)

// v0.9.0.2: MCM Dashboard aggregator. Backend fans out parallel probes
// to every registered cluster; one round-trip, capped at ~5s per cluster.
export const getMultiClusterSummary  = () => api.get('/multicluster/summary')

// v0.8.9.2: Applications-page one-click Snapshot button. Distinct from
// createBackup — this hits a narrow first-class endpoint with zero
// config; the backend hard-codes "cluster-local CSI snapshot, 24h TTL,
// default BSL" so the UI doesn't have to encode policy + safety
// defaults (which kept drifting from backend intent in v0.7).
// Body is optional ({ ttl, storageLocation, comment }) — caller can
// POST with no body at all.
export const createManualSnapshot = (namespace, data = {}) =>
  api.post(`/applications/${namespace}/snapshot`, data)
