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
  return config
})

export const getStatus = () => api.get('/status')
export const getDashboardSummary = () => api.get('/dashboard/summary')
export const getNamespaces = () => api.get('/namespaces')
export const getApplications = () => api.get('/applications')
export const getApplicationDetail = (namespace) => api.get(`/applications/${namespace}/details`)

// Backups
export const getBackups = () => api.get('/backups')
export const getBackup = (name) => api.get(`/backups/${name}`)
export const getBackupResources = (name) => api.get(`/backups/${name}/resources`)
export const getBackupLogs = (name) => api.get(`/backups/${name}/logs`)
export const createBackup = (data) => api.post('/backups', data)
export const deleteBackup = (name) => api.delete(`/backups/${name}`)

// Restores
export const getRestores = () => api.get('/restores')
export const getRestore = (name) => api.get(`/restores/${name}`)
export const createRestore = (data) => api.post('/restores', data)
export const deleteRestore = (name) => api.delete(`/restores/${name}`)
export const getRestoreResults = (name) => api.get(`/restores/${name}/results`)

// Schedules
export const getSchedules = () => api.get('/schedules')
export const createSchedule = (data) => api.post('/schedules', data)
export const patchSchedule = (name, data) => api.patch(`/schedules/${name}`, data)
export const deleteSchedule = (name) => api.delete(`/schedules/${name}`)

// Storage locations
export const getStorageLocations = () => api.get('/storage-locations')
export const getStorageLocation = (name) => api.get(`/storage-locations/${name}`)
export const createStorageLocation = (data) => api.post('/storage-locations', data)
export const updateStorageLocation = (name, data) => api.put(`/storage-locations/${name}`, data)
export const deleteStorageLocation = (name) => api.delete(`/storage-locations/${name}`)
export const verifyStorageLocation = (name) => api.post(`/storage-locations/${name}/verify`)
