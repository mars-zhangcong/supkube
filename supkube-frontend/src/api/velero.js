import axios from 'axios'

const BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1'

const api = axios.create({ baseURL: BASE_URL })

export const getStatus = () => api.get('/status')
export const getNamespaces = () => api.get('/namespaces')

// Backups
export const getBackups = () => api.get('/backups')
export const getBackup = (name) => api.get(`/backups/${name}`)
export const createBackup = (data) => api.post('/backups', data)
export const deleteBackup = (name) => api.delete(`/backups/${name}`)

// Restores
export const getRestores = () => api.get('/restores')
export const getRestore = (name) => api.get(`/restores/${name}`)
export const createRestore = (data) => api.post('/restores', data)

// Schedules
export const getSchedules = () => api.get('/schedules')
export const createSchedule = (data) => api.post('/schedules', data)
export const deleteSchedule = (name) => api.delete(`/schedules/${name}`)

// Storage locations
export const getStorageLocations = () => api.get('/storage-locations')
export const createStorageLocation = (data) => api.post('/storage-locations', data)
