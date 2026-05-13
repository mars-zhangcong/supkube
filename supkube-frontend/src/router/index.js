import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', name: 'Dashboard', component: Dashboard },
  { path: '/backups', name: 'Backups', component: () => import('../views/Backups.vue') },
  { path: '/restores', name: 'Restores', component: () => import('../views/Restores.vue') },
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
