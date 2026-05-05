<template>
  <nav class="w-64 min-h-screen glass-dark flex flex-col text-slate-100 relative z-20">
    <div class="px-6 py-8 border-b border-white/5 flex items-center gap-3">
      <div class="w-8 h-8 rounded-xl bg-gradient-to-br from-primary-400 to-primary-600 shadow-glow flex items-center justify-center shrink-0">
        <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
      </div>
      <span class="text-xl font-bold tracking-wide text-white">PigeonRelay</span>
    </div>

    <div class="flex-1 py-6 space-y-1.5 px-4 overflow-y-auto">
      <NavItem to="/" label="仪表盘" icon="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1" />
      <NavItem to="/nodes" label="节点管理" icon="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
      <NavItem to="/local-opt" label="本地优选" icon="M13 10V3L4 14h7v7l9-11h-7z" />
      <NavItem to="/traffic" label="延迟监控" icon="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />

      <div class="pt-6 pb-2">
        <p class="px-3 text-xs font-semibold text-primary-200 uppercase tracking-wider opacity-70">测速</p>
      </div>
      <NavItem to="/speed-clients" label="测速客户端" icon="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
      <NavItem to="/algorithms" label="优选算法" icon="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />

      <div class="pt-6 pb-2">
        <p class="px-3 text-xs font-semibold text-primary-200 uppercase tracking-wider opacity-70">系统</p>
      </div>
      <NavItem to="/settings" label="系统设置" icon="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      <NavItem to="/backups" label="配置备份" icon="M4 7v10c0 2 1 3 3 3h10c2 0 3-1 3-3V7M4 7c0-2 1-3 3-3h10c2 0 3 1 3 3M4 7h16M9 11v4m3-4v4m3-4v4" />
      <NavItem to="/sub-links" label="订阅短链" icon="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
    </div>

    <div class="p-4 border-t border-white/10 mt-auto bg-black/10">
      <div class="flex items-center gap-3 px-2 py-1 mb-4">
        <div class="w-8 h-8 rounded-full bg-primary-700 border border-primary-500 flex items-center justify-center text-sm font-bold shadow-inner">
          {{ auth.user?.username?.charAt(0).toUpperCase() || 'U' }}
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-medium text-white truncate">{{ auth.user?.username || 'User' }}</p>
          <p class="text-xs text-primary-300 truncate">{{ auth.isAdmin ? 'Administrator' : 'User' }}</p>
        </div>
      </div>
      <button @click="logout" class="w-full flex items-center justify-center gap-2 py-2 px-3 rounded-lg text-slate-300 hover:text-white hover:bg-white/10 transition-colors text-sm">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
        </svg>
        退出登录
      </button>
    </div>
  </nav>
</template>

<script setup>
import { useRouter } from 'vue-router'
import NavItem from './NavItem.vue'
import { useAuthStore } from '../stores/auth.js'

const router = useRouter()
const auth = useAuthStore()

function logout() {
  auth.logout()
  router.push('/login')
}
</script>
