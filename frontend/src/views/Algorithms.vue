<template>
  <div class="max-w-4xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">优选算法</h1>
        <p class="text-sm text-slate-500 mt-1">创建自定义算法，绑定算法类型与测速客户端筛选。</p>
      </div>
      <Button icon="M12 4v16m8-8H4" @click="openAdd">添加算法</Button>
    </div>

    <div v-if="loading" class="space-y-4">
      <Card v-for="i in 2" :key="i" class="animate-pulse"><div class="h-5 bg-slate-200 rounded w-1/3 mb-2"></div></Card>
    </div>

    <Card v-else-if="algorithms.length === 0" class="text-center py-16 text-slate-400">
      暂无自定义算法 — 点击右上角创建第一个
    </Card>

    <TransitionGroup v-else name="list" tag="div" class="space-y-3">
      <Card v-for="a in algorithms" :key="a.id" class="group">
        <div class="flex items-center gap-4">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-bold text-slate-800">{{ a.name }}</span>
              <Badge :variant="typeVariant(a.algo_type)">{{ typeLabel(a.algo_type) }}</Badge>
              <Badge variant="info">{{ a.client_filter === 'all' ? '全部客户端' : (clientCount(a.client_filter) + ' 个客户端') }}</Badge>
            </div>
          </div>
          <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button size="sm" variant="secondary" @click="openEdit(a)">编辑</Button>
            <Button size="sm" variant="danger" @click="doDelete(a)">删除</Button>
          </div>
        </div>
      </Card>
    </TransitionGroup>

    <Modal v-model="showForm" :title="isEdit ? '编辑算法' : '添加算法'">
      <div class="space-y-4">
        <div><label class="block text-sm font-medium text-slate-700 mb-1">算法名称</label><Input v-model="form.name" placeholder="如: 联通极速" /></div>
        <div><label class="block text-sm font-medium text-slate-700 mb-1">算法类型</label>
          <select v-model="form.algo_type" class="input-glass">
            <option value="fast">极速 (最低延迟)</option>
            <option value="steady">稳如磐石 (最稳定)</option>
            <option value="tidal">时间潮汐 (时间段加权)</option>
            <option value="composite">综合复合 (极速+稳定)</option>
            <option value="lowjitter">低抖动 (VoIP/直播)</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">绑定的测速客户端</label>
          <div class="flex items-center gap-2 mb-2">
            <input type="checkbox" v-model="useAllClients" @change="onAllToggle" />
            <label class="text-sm text-slate-600 cursor-pointer" @click="useAllClients = !useAllClients; onAllToggle()">全部客户端</label>
          </div>
          <div v-if="!useAllClients" class="max-h-48 overflow-y-auto border border-slate-200 rounded-lg p-2 space-y-1">
            <div v-for="c in speedClients" :key="c.client_id" class="flex items-center gap-2 text-sm">
              <input type="checkbox" :value="c.client_id" v-model="selectedClients" />
              <span class="font-mono text-xs text-slate-600 truncate">{{ c.client_id }}</span>
              <span v-if="c.name" class="text-xs text-slate-500">{{ c.name }}</span>
            </div>
            <div v-if="speedClients.length === 0" class="text-xs text-slate-400 p-2">暂无测速客户端</div>
          </div>
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showForm = false">取消</Button>
        <Button variant="primary" @click="save" :loading="saving">{{ isEdit ? '更新' : '创建' }}</Button>
      </template>
    </Modal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'
import Card from '../components/ui/Card.vue'
import Button from '../components/ui/Button.vue'
import Input from '../components/ui/Input.vue'
import Badge from '../components/ui/Badge.vue'
import Modal from '../components/ui/Modal.vue'

const algorithms = ref([])
const speedClients = ref([])
const loading = ref(true)
const saving = ref(false)
const showForm = ref(false)
const isEdit = ref(false)
const editingId = ref(null)
const useAllClients = ref(true)
const selectedClients = ref([])
const form = ref({ name: '', algo_type: 'fast' })

const TYPES = { fast: '极速', steady: '稳如磐石', tidal: '时间潮汐', composite: '综合复合', lowjitter: '低抖动' }
const VARIANTS = { fast: 'primary', steady: 'info', tidal: 'warning', composite: 'success', lowjitter: 'accent' }
function typeLabel(t) { return TYPES[t] || t }
function typeVariant(t) { return VARIANTS[t] || 'primary' }
function clientCount(f) { return f === 'all' ? 0 : f.split(',').length }

onMounted(load)

async function load() {
  try {
    const [alg, cli] = await Promise.all([api.listAlgorithms(), api.listSpeedClients().catch(() => [])])
    algorithms.value = alg || []
    speedClients.value = cli || []
  } catch { window.$toast?.error('加载失败') }
  finally { loading.value = false }
}

function onAllToggle() { if (useAllClients.value) selectedClients.value = [] }

function openAdd() {
  isEdit.value = false; editingId.value = null
  form.value = { name: '', algo_type: 'fast' }
  useAllClients.value = true; selectedClients.value = []
  showForm.value = true
}

function openEdit(a) {
  isEdit.value = true; editingId.value = a.id
  form.value = { name: a.name, algo_type: a.algo_type }
  if (a.client_filter === 'all') { useAllClients.value = true; selectedClients.value = [] }
  else { useAllClients.value = false; selectedClients.value = a.client_filter.split(',') }
  showForm.value = true
}

async function save() {
  if (!form.value.name) { window.$toast?.warning('名称必填'); return }
  saving.value = true
  const clientFilter = useAllClients.value ? 'all' : selectedClients.value.join(',')
  try {
    if (isEdit.value) {
      await api.updateAlgorithm(editingId.value, { name: form.value.name, algo_type: form.value.algo_type, client_filter: clientFilter })
    } else {
      await api.createAlgorithm({ name: form.value.name, algo_type: form.value.algo_type, client_filter: clientFilter })
      window.$toast?.success('已创建')
    }
    showForm.value = false
    await load()
  } catch (e) { window.$toast?.error('保存失败', e.message) }
  finally { saving.value = false }
}

async function doDelete(a) {
  if (!confirm(`确认删除算法 "${a.name}"？`)) return
  try { await api.deleteAlgorithm(a.id); window.$toast?.success('已删除'); await load() }
  catch (e) { window.$toast?.error('删除失败', e.message) }
}
</script>
