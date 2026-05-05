<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="mb-8">
      <h1 class="text-3xl font-bold text-slate-800 tracking-tight">仪表盘</h1>
      <p class="text-slate-500 mt-2">欢迎回来，以下是您的系统运行状态。<span class="text-xs text-slate-400 ml-2">v{{ version }}</span></p>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
      <Card class="hover:-translate-y-1 hover:shadow-xl transition-all duration-300">
        <div class="flex items-center justify-between mb-4">
          <div class="w-12 h-12 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
          </div>
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Node Groups</span>
        </div>
        <p class="text-sm font-medium text-slate-500">节点分组</p>
        <div class="mt-2 flex items-baseline gap-2">
          <span v-if="loading" class="h-10 w-16 bg-slate-200 rounded animate-pulse"></span>
          <span v-else class="text-4xl font-extrabold text-slate-800">{{ stats.groups }}</span>
        </div>
      </Card>

      <Card class="hover:-translate-y-1 hover:shadow-xl transition-all duration-300">
        <div class="flex items-center justify-between mb-4">
          <div class="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
          </div>
          <span class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Sources</span>
        </div>
        <p class="text-sm font-medium text-slate-500">订阅源数量</p>
        <div class="mt-2 flex items-baseline gap-2">
          <span v-if="loading" class="h-10 w-16 bg-slate-200 rounded animate-pulse"></span>
          <span v-else class="text-4xl font-extrabold text-slate-800">{{ stats.sources }}</span>
        </div>
      </Card>

      <Card class="hover:-translate-y-1 hover:shadow-xl transition-all duration-300 relative overflow-hidden group">
        <div class="absolute -right-6 -top-6 w-32 h-32 bg-gradient-to-br from-primary-400/20 to-accent/20 rounded-full blur-2xl group-hover:scale-150 transition-transform duration-700"></div>
        <div class="relative z-10">
          <div class="flex items-center justify-between mb-4">
            <div class="w-12 h-12 rounded-xl bg-primary-50 text-primary-600 flex items-center justify-center">
              <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <span class="text-xs font-semibold text-primary-400 uppercase tracking-wider">Traffic</span>
          </div>
          <p class="text-sm font-medium text-slate-500">本月已用流量</p>
          <div class="mt-2 flex items-baseline gap-2">
            <span v-if="loading" class="h-10 w-24 bg-slate-200 rounded animate-pulse"></span>
            <span v-else class="text-4xl font-extrabold text-primary-700">{{ formatBytes(stats.usedBytes).split(' ')[0] }}</span>
            <span v-if="!loading" class="text-lg font-semibold text-primary-500">{{ formatBytes(stats.usedBytes).split(' ')[1] }}</span>
          </div>
          <div v-if="!loading && stats.quotaBytes > 0" class="mt-3">
            <div class="w-full bg-slate-100 rounded-full h-1.5 overflow-hidden">
              <div
                class="h-1.5 rounded-full transition-all duration-1000 ease-out"
                :class="usagePct > 80 ? 'bg-danger' : usagePct > 60 ? 'bg-warning' : 'bg-primary-500'"
                :style="{ width: `${Math.min(usagePct, 100)}%` }"
              ></div>
            </div>
            <p class="text-xs text-slate-400 mt-1.5 text-right">{{ usagePct }}% of {{ formatBytes(stats.quotaBytes) }}</p>
          </div>
        </div>
      </Card>
    </div>

    <Card>
      <div class="flex items-center justify-between mb-6">
        <h2 class="text-lg font-bold text-slate-800">快速操作</h2>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
        <router-link to="/groups" class="group p-4 rounded-xl border border-slate-100 hover:border-primary-200 hover:bg-primary-50 transition-all flex flex-col items-center justify-center text-center gap-3">
          <div class="w-10 h-10 rounded-full bg-primary-100 text-primary-600 flex items-center justify-center group-hover:scale-110 group-hover:bg-primary-500 group-hover:text-white transition-all">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" /></svg>
          </div>
          <span class="text-sm font-medium text-slate-700 group-hover:text-primary-700">新建节点分组</span>
        </router-link>

        <router-link to="/sources" class="group p-4 rounded-xl border border-slate-100 hover:border-indigo-200 hover:bg-indigo-50 transition-all flex flex-col items-center justify-center text-center gap-3">
          <div class="w-10 h-10 rounded-full bg-indigo-100 text-indigo-600 flex items-center justify-center group-hover:scale-110 group-hover:bg-indigo-500 group-hover:text-white transition-all">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" /></svg>
          </div>
          <span class="text-sm font-medium text-slate-700 group-hover:text-indigo-700">添加订阅源</span>
        </router-link>

        <router-link to="/sub-links" class="group p-4 rounded-xl border border-slate-100 hover:border-accent/30 hover:bg-sky-50 transition-all flex flex-col items-center justify-center text-center gap-3">
          <div class="w-10 h-10 rounded-full bg-sky-100 text-sky-600 flex items-center justify-center group-hover:scale-110 group-hover:bg-sky-500 group-hover:text-white transition-all">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" /></svg>
          </div>
          <span class="text-sm font-medium text-slate-700 group-hover:text-sky-700">生成短链</span>
        </router-link>

        <router-link to="/backups" class="group p-4 rounded-xl border border-slate-100 hover:border-emerald-200 hover:bg-emerald-50 transition-all flex flex-col items-center justify-center text-center gap-3">
          <div class="w-10 h-10 rounded-full bg-emerald-100 text-emerald-600 flex items-center justify-center group-hover:scale-110 group-hover:bg-emerald-500 group-hover:text-white transition-all">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" /></svg>
          </div>
          <span class="text-sm font-medium text-slate-700 group-hover:text-emerald-700">配置备份</span>
        </router-link>
      </div>
    </Card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { api } from '../api.js'
import Card from '../components/ui/Card.vue'

const version = ref('')
const stats = ref({ sources: 0, groups: 0, usedBytes: 0, quotaBytes: 0 })
const loading = ref(true)

const usagePct = computed(() => {
  if (!stats.value.quotaBytes) return 0
  return Math.round((stats.value.usedBytes / stats.value.quotaBytes) * 100)
})

onMounted(async () => {
  loading.value = true
  try {
    const [sources, groups, traffic] = await Promise.all([
      api.listSources(),
      api.listGroups(),
      api.trafficOverview(),
    ])
    stats.value.sources = sources?.length || 0
    stats.value.groups = groups?.length || 0
    const s = await api.request('GET', '/settings').catch(() => ({}))
    version.value = s.version || ''
    stats.value.usedBytes = traffic?.used_bytes || 0
    stats.value.quotaBytes = traffic?.quota_bytes || 0
  } catch (e) {
    if (window.$toast) window.$toast.error('数据加载失败', '请检查网络连接')
  } finally {
    loading.value = false
  }
})

function formatBytes(b) {
  if (!b) return '0 B'
  if (b >= 1 << 30) return (b / (1 << 30)).toFixed(1) + ' GB'
  if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + ' MB'
  if (b >= 1 << 10) return (b / (1 << 10)).toFixed(1) + ' KB'
  return b + ' B'
}
</script>
