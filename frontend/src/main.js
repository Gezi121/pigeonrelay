import './style.css'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('./views/Login.vue'),
  },
  {
    path: '/',
    name: 'dashboard',
    component: () => import('./views/Dashboard.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/nodes',
    name: 'nodes',
    component: () => import('./views/Nodes.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/local-opt',
    name: 'localopt',
    component: () => import('./views/LocalOpt.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/traffic',
    name: 'traffic',
    component: () => import('./views/Traffic.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('./views/Settings.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/backups',
    name: 'backups',
    component: () => import('./views/Backups.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/speed-clients',
    name: 'speedclients',
    component: () => import('./views/SpeedClients.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/algorithms',
    name: 'algorithms',
    component: () => import('./views/Algorithms.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/sub-links',
    name: 'sublinks',
    component: () => import('./views/SubLinks.vue'),
    meta: { requiresAuth: true },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const token = localStorage.getItem('token')
  if (to.meta.requiresAuth && !token) return '/login'
  if (to.path === '/login' && token) return '/'
})

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
