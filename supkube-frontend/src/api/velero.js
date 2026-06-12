import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

export default api

export function getAuthProviders() {
  return api.get('/auth/providers')
}

export function authCallback(params) {
  return api.get('/auth/callback', { params })
}

export function getAuthMe() {
  return api.get('/auth/me')
}

export function authLogout() {
  return api.post('/auth/logout')
}

export function getRestorePoints(params = {}) {
  return api.get('/restore-points', { params })
}
