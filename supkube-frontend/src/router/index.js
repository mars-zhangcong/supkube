import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import { useAuth } from '../composables/useAuth'

const routes = [
  // v0.8.5: Login route — public, no guard.
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue'), meta: { public: true } },
  // Same component handles the OIDC callback (Dex redirects here with ?code).
  { path: '/auth/callback', name: 'AuthCallback', component: () => import('../views/Login.vue'), meta: { public: true } },

  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard },
  // v0.9.0.2: MCM Dashboard — the page Mode Switcher → "Multi-Cluster
  // Manager" lands on. Aggregated view across all registered clusters.
  { path: '/multicluster', name: 'Multicluster', component: () => import('../views/MulticlusterDashboard.vue') },
  { path: '/applications', name: 'Applications', component: () => import('../views/Applications.vue') },
  { path: '/backups', name: 'Backups', component: () => import('../views/Backups.vue') },
  { path: '/backups/:name', name: 'BackupDetail', component: () => import('../views/BackupDetail.vue') },
  // v0.8.0: Activity replaces the Restores page. Keep /restores as a
  // redirect for any bookmarks/external links that still point at it.
  { path: '/activity', name: 'Activity', component: () => import('../views/Activity.vue') },
  { path: '/restores', redirect: '/activity?type=Restore' },
  { path: '/policies', name: 'Policies', component: () => import('../views/Policies.vue') },
  { path: '/transform-sets', name: 'TransformSets', component: () => import('../views/TransformSets.vue') },
  { path: '/storage', name: 'StorageLocations', component: () => import('../views/StorageLocations.vue') },
  { path: '/snapshot-locations', name: 'VolumeSnapshotLocations', component: () => import('../views/VolumeSnapshotLocations.vue') },
  { path: '/advisor', name: 'BackupAdvisor', component: () => import('../views/BackupAdvisor.vue') },
  { path: '/settings', name: 'Settings', component: () => import('../views/Settings.vue') },
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// v0.8.5 navigation guard. We need useAuth() to be initialized before we
// can read isLoggedIn — so we bootstrap once and then gate every
// subsequent navigation. Public routes (login, callback) always pass.
let authBootstrapped = false
router.beforeEach(async (to) => {
  if (to.meta?.public) return true

  const auth = useAuth()
  if (!authBootstrapped) {
    await auth.bootstrap()
    authBootstrapped = true
  }
  if (auth.isLoggedIn.value) return true
  // Not logged in → bounce to /login. Preserve where they were trying
  // to go so we can return them there after login (future v0.8.6).
  return { path: '/login', query: { redirect: to.fullPath } }
})

export default router
