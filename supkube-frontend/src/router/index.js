import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard },
  { path: '/backups', name: 'Backups', component: () => import('../views/Backups.vue') },
  { path: '/backups/:name', name: 'BackupDetail', component: () => import('../views/BackupDetail.vue') },
  { path: '/restores', name: 'Restores', component: () => import('../views/Restores.vue') },
  { path: '/policies', name: 'Policies', component: () => import('../views/Policies.vue') },
  { path: '/storage', name: 'StorageLocations', component: () => import('../views/StorageLocations.vue') },
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
