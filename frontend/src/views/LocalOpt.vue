<template>
  <div class="max-w-4xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">本地优选看板</h1>
        <p class="text-sm text-slate-500 mt-1">管理本地优选域名与第三方优选域名，查看实时最优 IP。</p>
      </div>
      <div class="flex gap-2">
        <Button variant="secondary" icon="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" @click="openBatch">批量添加</Button>
        <Button icon="M12 4v16m8-8H4" @click="openAdd">添加域名</Button>
      </div>
    </div>

    <div v-if="loading" class="space-y-6">
      <div v-for="label in ['本地优选', '第三方优选']" :key="label">
        <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-3">{{ label }}</h2>
        <Card v-for="i in 2" :key="i" class="animate-pulse mb-3"><div class="h-5 bg-slate-200 rounded w-1/3 mb-2"></div><div class="h-4 bg-slate-100 rounded w-1/2"></div></Card>
      </div>
    </div>

    <Card v-else-if="mappings.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
      </div>
      <p class="text-slate-600 font-medium mb-1">暂无优选域名</p>
      <p class="text-sm text-slate-400 mb-6">添加本地或第三方 CF 优选域名</p>
      <Button @click="openAdd">添加第一个域名</Button>
    </Card>

    <template v-else>
      <!-- 本地优选 -->
      <div class="mb-8">
        <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-3">本地优选</h2>
        <Card v-if="localItems.length === 0" class="text-center py-8 text-sm text-slate-400">暂无本地优选域名</Card>
        <TransitionGroup v-else name="list" tag="div" class="space-y-3">
          <Card v-for="dm in localItems" :key="dm.id" class="group">
            <div class="flex items-center gap-4">
              <div class="flex-1 min-w-0 flex items-center gap-2">
                <span class="font-bold text-slate-800 font-mono">{{ dm.subdomain }}</span>
                <span class="text-xs text-slate-400 font-mono truncate">{{ dm.full_domain }}</span>
              </div>
              <select class="input-glass w-32 text-xs py-1.5" :value="dm.algorithm_type" @change="setAlgorithm(dm, $event.target.value)">
                <optgroup label="内置">
                  <option value="fast">极速</option><option value="steady">稳如磐石</option><option value="tidal">时间潮汐</option><option value="composite">综合复合</option><option value="lowjitter">低抖动</option>
                </optgroup>
                <optgroup v-if="customAlgos.length" label="自定义">
                  <option v-for="a in customAlgos" :key="a.name" :value="a.name">{{ a.name }}</option>
                </optgroup>
              </select>
              <div class="flex-1 bg-slate-50 rounded-lg px-3 py-1.5 border border-slate-100 flex items-center gap-3 text-xs min-w-0">
                <template v-if="bestIPs[dm.id]?.ip && bestIPs[dm.id].ip !== '—'">
                  <span class="font-mono font-bold text-primary-700 shrink-0">{{ bestIPs[dm.id].ip }}</span>
                  <span class="text-slate-500 shrink-0">{{ bestIPs[dm.id].avg_ms }}ms</span>
                  <span class="shrink-0" :class="bestIPs[dm.id].success_rt >= 80 ? 'text-emerald-600' : 'text-red-500'">↑{{ bestIPs[dm.id].success_rt }}%</span>
                  <span class="text-slate-400 shrink-0">σ{{ bestIPs[dm.id].stddev }}ms</span>
                  <span class="text-amber-600 shrink-0" v-if="bestIPs[dm.id].unstable > 0">{{ bestIPs[dm.id].unstable }}不稳</span>
                  <span class="text-slate-400 truncate" :title="bestIPs[dm.id].algo_type">{{ bestIPs[dm.id].algo_type }}</span>
                </template>
                <span v-else class="text-slate-400">—</span>
                <span v-if="ipLoading[dm.id]" class="ml-auto animate-spin text-slate-300">⟳</span>
              </div>
              <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                <button class="btn-secondary text-xs py-1 px-2" @click="editDM(dm)">编辑</button>
                <button class="btn-secondary text-xs py-1 px-2" @click="refreshIP(dm)" :disabled="ipLoading[dm.id]">
                  <svg class="w-4 h-4" :class="ipLoading[dm.id] ? 'animate-spin' : ''" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                </button>
                <button class="btn-danger text-xs py-1 px-2" @click="doDelete(dm)">删除</button>
              </div>
            </div>
          </Card>
        </TransitionGroup>
      </div>

      <!-- 第三方优选 -->
      <div>
        <h2 class="text-sm font-semibold text-slate-400 uppercase tracking-wider mb-3">第三方优选</h2>
        <Card v-if="externalItems.length === 0" class="text-center py-8 text-sm text-slate-400">暂无第三方优选域名</Card>
        <TransitionGroup v-else name="list" tag="div" class="space-y-3">
          <Card v-for="dm in externalItems" :key="dm.id" class="group">
            <div class="flex items-center gap-4">
              <div class="flex-1 min-w-0 flex items-center gap-2">
                <span class="font-bold text-slate-800 font-mono truncate">{{ dm.full_domain }}</span>
                <Badge variant="info">第三方</Badge>
              </div>
              <div class="w-48 bg-slate-50 rounded-lg px-3 py-1.5 border border-slate-100">
                <div class="flex items-center gap-2">
                  <span :class="pings[dm.id]?.reachable ? 'text-success' : 'text-danger'" class="text-xs font-bold">{{ pings[dm.id]?.reachable ? '可达' : pings[dm.id] === undefined ? '—' : '不可达' }}</span>
                  <span v-if="pings[dm.id]?.ip" class="font-mono text-xs text-slate-500">{{ pings[dm.id].ip }}</span>
                  <span v-if="pings[dm.id]?.latency_ms" class="text-xs text-slate-400">{{ pings[dm.id].latency_ms }}ms</span>
                </div>
              </div>
              <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                <button class="btn-secondary text-xs py-1 px-2" @click="editDM(dm)">编辑</button>
                <button class="btn-secondary text-xs py-1 px-2" @click="pingExt(dm)" :disabled="ipLoading[dm.id]">
                  <svg class="w-4 h-4" :class="ipLoading[dm.id] ? 'animate-spin' : ''" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                </button>
                <button class="btn-danger text-xs py-1 px-2" @click="doDelete(dm)">删除</button>
              </div>
            </div>
          </Card>
        </TransitionGroup>
      </div>
    </template>

    <!-- 批量添加弹窗 -->
    <Modal v-model="showBatch" title="批量添加域名">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">来源类型</label>
          <div class="flex gap-2">
            <button @click="batchForm.source = 'local'" :class="batchForm.source === 'local' ? 'btn-primary' : 'btn-secondary'" class="flex-1 py-2 text-sm rounded-xl">本地优选</button>
            <button @click="batchForm.source = 'external'" :class="batchForm.source === 'external' ? 'btn-primary' : 'btn-secondary'" class="flex-1 py-2 text-sm rounded-xl">第三方优选</button>
          </div>
        </div>
        <template v-if="batchForm.source === 'local'">
          <div><label class="block text-sm font-medium text-slate-700 mb-1">子域名（一行一个）</label><textarea v-model="batchForm.subdomains" rows="5" class="input-glass font-mono text-xs" placeholder="dshax1&#10;dshax2&#10;dshax3"></textarea></div>
          <div><label class="block text-sm font-medium text-slate-700 mb-1">后段域名</label><Input v-model="batchForm.suffix" placeholder=".your-domain.com" /><p class="text-xs text-slate-400 mt-1">预览: <span class="font-mono text-primary-600">{{ batchPreview }}</span></p></div>
        </template>
        <div v-else><label class="block text-sm font-medium text-slate-700 mb-1">完整域名（一行一个）</label><textarea v-model="batchForm.domains" rows="5" class="input-glass font-mono text-xs" placeholder="cf.090227.xyz&#10;cf2.example.com"></textarea></div>
        <div class="text-xs text-slate-500 bg-slate-50 p-3 rounded-lg">预计创建 <span class="font-bold text-slate-800">{{ batchCount }}</span> 个域名</div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showBatch = false">取消</Button>
        <Button variant="primary" @click="doBatchCreate" :loading="saving" :disabled="batchCount === 0">批量创建 {{ batchCount }} 个</Button>
      </template>
    </Modal>

    <Modal v-model="showForm" :title="isEdit ? '编辑域名' : '添加优选域名'">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">来源类型</label>
          <div class="flex gap-2">
            <button @click="form.source = 'local'" :class="form.source === 'local' ? 'btn-primary' : 'btn-secondary'" class="flex-1 py-2 text-sm rounded-xl">本地优选</button>
            <button @click="form.source = 'external'" :class="form.source === 'external' ? 'btn-primary' : 'btn-secondary'" class="flex-1 py-2 text-sm rounded-xl">第三方优选</button>
          </div>
        </div>
        <div v-if="form.source === 'local'">
          <label class="block text-sm font-medium text-slate-700 mb-1">子域名</label>
          <Input v-model="form.subdomain" placeholder="例如: dshax1" />
          <p class="text-xs text-slate-400 mt-1">完整域名将为 <span class="font-mono text-primary-600">{{ form.subdomain || 'dshax1' }}.your-domain.com</span></p>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">{{ form.source === 'external' ? '域名' : '完整域名' }}</label>
          <Input v-model="form.full_domain" :placeholder="form.source === 'external' ? 'cf.090227.xyz' : 'dshax1.your-domain.com'" />
        </div>
        <div v-if="form.source === 'local'">
          <label class="block text-sm font-medium text-slate-700 mb-1">算法</label>
          <select v-model="form.algorithm_type" class="input-glass">
            <optgroup label="内置">
              <option value="fast">极速</option><option value="steady">稳如磐石</option><option value="tidal">时间潮汐</option><option value="composite">综合复合</option><option value="lowjitter">低抖动</option>
            </optgroup>
            <optgroup v-if="customAlgos.length" label="自定义">
              <option v-for="a in customAlgos" :key="a.name" :value="a.name">{{ a.name }}</option>
            </optgroup>
          </select>
        </div>
        <p v-else class="text-xs text-slate-400 bg-slate-50 p-3 rounded-lg">第三方域名直接做连通性检测，无需子域名和算法配置。</p>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showForm = false">取消</Button>
        <Button variant="primary" @click="save" :loading="saving">{{ isEdit ? '更新' : '添加' }}</Button>
      </template>
    </Modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { api } from '../api.js'
import Card from '../components/ui/Card.vue'
import Button from '../components/ui/Button.vue'
import Input from '../components/ui/Input.vue'
import Badge from '../components/ui/Badge.vue'
import Modal from '../components/ui/Modal.vue'

const mappings = ref([])
const customAlgos = ref([])
const bestIPs = ref({})
const ipLoading = ref({})
const loading = ref(true)
const saving = ref(false)
const showForm = ref(false)
let refreshAbort = null  // AbortController for refreshAllIPs
const isEdit = ref(false)
const editingId = ref(null)
const form = ref({ subdomain: '', full_domain: '', algorithm_type: 'fast', source: 'local' })

const localItems = computed(() => mappings.value.filter(d => d.source !== 'external'))
const externalItems = computed(() => mappings.value.filter(d => d.source === 'external'))

watch(() => form.value.subdomain, (sub) => {
  if (form.value.source === 'local' && sub) {
    form.value.full_domain = sub + '.your-domain.com'
  }
})

onMounted(load)
onBeforeUnmount(() => {
  if (refreshAbort) { refreshAbort.abort(); refreshAbort = null }
})

async function load() {
  const firstLoad = loading.value
  try {
    const [dm, algos] = await Promise.all([
      api.listDomainMappings(),
      api.listAlgorithms().catch(() => [])
    ])
    mappings.value = dm || []
    customAlgos.value = algos || []
    // Don't block UI — refresh IPs in background
    refreshAllIPs()
  } catch {
    window.$toast?.error('加载失败', '无法获取域名列表')
  } finally {
    loading.value = false
  }
}

const showBatch = ref(false)
const STORAGE_KEY = 'localopt_batch_last'
const batchForm = ref(loadBatchMemory())

function loadBatchMemory() {
  try {
    const s = localStorage.getItem(STORAGE_KEY)
    if (s) return JSON.parse(s)
  } catch {}
  return { source: 'local', subdomains: '', suffix: '.your-domain.com', domains: '' }
}

function saveBatchMemory() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(batchForm.value))
}

const batchPreview = computed(() => {
  const lines = batchForm.value.subdomains.trim().split('\n').filter(l => l.trim()).slice(0, 3)
  return lines.map(l => l.trim() + batchForm.value.suffix).join(', ')
})

const batchCount = computed(() => {
  if (batchForm.value.source === 'external') {
    return batchForm.value.domains.trim().split('\n').filter(l => l.trim()).length
  }
  return batchForm.value.subdomains.trim().split('\n').filter(l => l.trim()).length
})

function openBatch() {
  batchForm.value = loadBatchMemory()
  showBatch.value = true
}

async function doBatchCreate() {
  saving.value = true
  try {
    const data = { source: batchForm.value.source }
    if (data.source === 'external') {
      data.domains = batchForm.value.domains.trim().split('\n').filter(l => l.trim())
    } else {
      data.subdomains = batchForm.value.subdomains.trim().split('\n').filter(l => l.trim())
      data.suffix = batchForm.value.suffix
    }
    const list = await api.batchCreateDomainMappings(data)
    window.$toast?.success('批量添加成功', `已创建 ${list.length} 个域名`)
    saveBatchMemory()
    showBatch.value = false
    await load()
  } catch (e) { window.$toast?.error('批量添加失败', e.message) }
  finally { saving.value = false }
}

const pings = ref({})

async function refreshAllIPs() {
  if (refreshAbort) refreshAbort.abort()
  refreshAbort = new AbortController()
  const signal = refreshAbort.signal
  const tasks = mappings.value.map(dm =>
    dm.source === 'external' ? pingExt(dm) : refreshIP(dm, signal)
  )
  await Promise.all(tasks).catch(() => {})
}

async function pingExt(dm) {
  ipLoading.value[dm.id] = true
  try {
    const res = await api.pingDomain(dm.full_domain)
    pings.value[dm.id] = { reachable: res.reachable, ip: res.ip, latency_ms: res.latency_ms }
  } catch {
    pings.value[dm.id] = { reachable: false, ip: '', latency_ms: 0 }
  } finally {
    ipLoading.value[dm.id] = false
  }
}

async function refreshIP(dm, signal) {
  if (signal?.aborted) return
  ipLoading.value[dm.id] = true
  try {
    const res = await api.latencyBestDetail(dm.full_domain)
    if (signal?.aborted) return
    bestIPs.value[dm.id] = {
      ip: res.ip, avg_ms: res.avg_ms, score: res.score,
      success_rt: res.success_rt, stable_rt: res.stable_rt,
      stddev: res.stddev, unstable: res.unstable, total: res.total,
      algo_type: res.algo_type,
    }
  } catch {
    if (!signal?.aborted) bestIPs.value[dm.id] = { ip: '—', avg_ms: 0 }
  } finally {
    if (!signal?.aborted) ipLoading.value[dm.id] = false
  }
}

async function setAlgorithm(dm, algo) {
  try {
    await api.updateDomainMapping(dm.id, { algorithm_type: algo })
    dm.algorithm_type = algo
  } catch {
    window.$toast?.error('更新失败', '算法切换失败')
  }
}

function openAdd() { isEdit.value = false; editingId.value = null; form.value = { subdomain: '', full_domain: '', algorithm_type: 'fast', source: 'local' }; showForm.value = true }
function editDM(dm) { isEdit.value = true; editingId.value = dm.id; form.value = { subdomain: dm.subdomain, full_domain: dm.full_domain, algorithm_type: dm.algorithm_type, source: dm.source || 'local' }; showForm.value = true }

async function save() {
  const data = { ...form.value }
  if (data.source === 'external') {
    if (!data.full_domain) { window.$toast?.warning('表单不完整', '域名必填'); return }
    data.subdomain = data.full_domain // auto-fill for external
  }
  if (!data.subdomain || !data.full_domain) { window.$toast?.warning('表单不完整', '子域名和完整域名必填'); return }
  saving.value = true
  try {
    if (isEdit.value) {
      await api.updateDomainMapping(editingId.value, { subdomain: data.subdomain, full_domain: data.full_domain, algorithm_type: data.algorithm_type })
    } else {
      await api.createDomainMapping(data)
      window.$toast?.success('添加成功', '优选域名已创建')
    }
    showForm.value = false; await load()
  } catch (e) { window.$toast?.error('保存失败', e.message) } finally { saving.value = false }
}

async function doDelete(dm) {
  if (!confirm(`确认删除域名 "${dm.subdomain}"？`)) return
  try { await api.deleteDomainMapping(dm.id); window.$toast?.success('已删除'); await load() }
  catch (e) { window.$toast?.error('删除失败', e.message) }
}
</script>
