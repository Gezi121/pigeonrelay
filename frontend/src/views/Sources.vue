<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">订阅源管理</h1>
        <p class="text-sm text-slate-500 mt-1">管理并测试所有的 Clash、V2Ray 等订阅链接。</p>
      </div>
      <Button icon="M12 4v16m8-8H4" @click="openForm">添加订阅源</Button>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading" class="space-y-4">
      <Card v-for="i in 3" :key="i" class="animate-pulse">
        <div class="h-5 bg-slate-200 rounded w-1/4 mb-3"></div>
        <div class="h-4 bg-slate-100 rounded w-1/2 mb-2"></div>
        <div class="h-4 bg-slate-100 rounded w-1/6"></div>
      </Card>
    </div>

    <!-- 列表空态 -->
    <Card v-else-if="sources.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
      </div>
      <p class="text-slate-600 font-medium mb-1">暂无订阅源</p>
      <p class="text-sm text-slate-400 mb-6">点击右上角按钮添加您的第一个订阅链接</p>
      <Button variant="secondary" icon="M12 4v16m8-8H4" @click="showForm = true">添加订阅源</Button>
    </Card>

    <!-- 列表数据 -->
    <TransitionGroup v-else name="list" tag="div" class="grid gap-4">
      <Card v-for="s in sources" :key="s.id" class="group relative overflow-hidden">
        <div class="absolute left-0 top-0 bottom-0 w-1" :class="s.last_fetch_status === 'ok' ? 'bg-success' : (s.last_fetch_status ? 'bg-danger' : 'bg-warning')"></div>

        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0 pr-4">
            <div class="flex items-center gap-3 mb-1">
              <h3 class="font-bold text-slate-800 truncate">{{ s.name }}</h3>
              <Badge :variant="s.type === 'clash' ? 'primary' : 'warning'">{{ s.type.toUpperCase() }}</Badge>
              <Badge :dot="true" :variant="s.last_fetch_status === 'ok' ? 'success' : (s.last_fetch_status ? 'danger' : 'warning')">
                {{ s.last_fetch_status === 'ok' ? '已通' : (s.last_fetch_status ? '失败' : '未测') }}
              </Badge>
            </div>

            <div class="flex items-center gap-2 text-sm text-slate-500 mt-2">
              <svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
              </svg>
              <span class="truncate" :title="s.url">{{ s.url || '手动输入' }}</span>
              <span v-if="s.status" class="ml-2 text-xs font-mono px-2 py-0.5 rounded-md" :class="s.status === 'tested' ? 'bg-emerald-100 text-emerald-700' : (s.status === 'untested' ? 'bg-slate-100 text-slate-500' : 'bg-red-100 text-red-700')">
                [{{ s.status === 'tested' ? '已测试' : (s.status === 'untested' ? '未测' : '异常') }}{{ s.last_tested_at ? '(' + new Date(s.last_tested_at).toLocaleDateString([], {month:'numeric', day:'numeric'}) + ')' : '' }}]
              </span>
            </div>
          </div>

          <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button size="sm" variant="secondary" icon="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" @click="editSource(s)">编辑</Button>
            <Button size="sm" variant="secondary" icon="M13 10V3L4 14h7v7l9-11h-7z" @click="testSource(s)" :loading="testingId === s.id">测试</Button>
            <Button size="sm" variant="danger" icon="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" @click="deleteSource(s)">删除</Button>
          </div>
        </div>

        <div v-if="s.last_fetch_status && s.last_fetch_status !== 'ok'" class="mt-3 text-xs text-danger bg-danger/5 p-2 rounded-lg border border-danger/10">
          {{ s.last_fetch_status }}
        </div>
      </Card>
    </TransitionGroup>

    <!-- 新建/编辑弹窗 -->
    <Modal v-model="showForm" :title="isEdit ? '编辑订阅源' : '添加订阅源'">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">源名称</label>
          <Input v-model="form.name" placeholder="例如：我的主线订阅" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">内容类型</label>
          <select v-model="form.content_mode" class="input-glass">
            <option value="url">远程链接 (URL)</option>
            <option value="local">本地导入 (文本)</option>
          </select>
        </div>
        <div v-if="form.content_mode === 'url'">
          <label class="block text-sm font-medium text-slate-700 mb-1">解析类型</label>
          <select v-model="form.type" class="input-glass">
            <option value="clash">Clash YAML</option>
            <option value="v2ray">V2Ray Base64 (vmess/vless)</option>
          </select>
        </div>
        <div v-if="form.content_mode === 'url'">
          <label class="block text-sm font-medium text-slate-700 mb-1">订阅链接</label>
          <Input v-model="form.url" placeholder="https://" />
        </div>
        <div v-else>
          <label class="block text-sm font-medium text-slate-700 mb-1">本地节点内容</label>
          <textarea v-model="form.local_nodes" rows="5" class="input-glass font-mono text-xs" placeholder="粘贴 vmess://... 或 YAML 节点组"></textarea>
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showForm = false">取消</Button>
        <Button variant="primary" @click="saveSource" :loading="saving">保存配置</Button>
      </template>
    </Modal>

    <!-- 测试结果弹窗 -->
    <Modal v-model="showTestResult" :title="testData?.summary || '测试结果'">
      <div v-if="testData" class="space-y-3">
        <div class="flex items-center gap-3 bg-slate-50 p-3 rounded-xl border border-slate-100">
          <Badge :variant="testData.node_count > 0 ? 'success' : 'danger'">
            {{ testData.node_count > 0 ? `解析成功: ${testData.node_count} 节点` : '解析失败或无节点' }}
          </Badge>
          <Badge variant="primary">{{ testData.type }}</Badge>
        </div>

        <div class="max-h-64 overflow-y-auto rounded-xl border border-slate-100 bg-white">
          <table class="w-full text-xs text-left">
            <thead class="bg-slate-50 sticky top-0 border-b border-slate-100">
              <tr>
                <th class="px-3 py-2 font-medium text-slate-500">名称</th>
                <th class="px-3 py-2 font-medium text-slate-500">地址:端口</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              <tr v-for="n in testData.nodes" :key="n.name" class="hover:bg-slate-50/50">
                <td class="px-3 py-2 font-medium text-slate-700 truncate max-w-[150px]">{{ n.name }}</td>
                <td class="px-3 py-2 text-slate-500 font-mono">{{ n.server }}:{{ n.port }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <template #footer>
        <Button variant="primary" @click="showTestResult = false">关闭</Button>
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

const sources = ref([])
const loading = ref(true)
const saving = ref(false)
const showForm = ref(false)

const showTestResult = ref(false)
const testData = ref(null)
const testingId = ref(null)

const isEdit = ref(false)
const editingId = ref(null)
const form = ref({ content_mode: 'url', type: 'clash', url: '', name: '', local_nodes: '' })

function openForm() {
  isEdit.value = false
  editingId.value = null
  form.value = { content_mode: 'url', type: 'clash', url: '', name: '', local_nodes: '' }
  showForm.value = true
}

function editSource(s) {
  isEdit.value = true
  editingId.value = s.id
  form.value = {
    content_mode: s.local_nodes ? 'local' : 'url',
    type: s.type || 'clash',
    url: s.url || '',
    name: s.name || '',
    local_nodes: s.local_nodes || ''
  }
  showForm.value = true
}

onMounted(load)

async function load() {
  try {
    sources.value = await api.listSources()
  } catch {
    window.$toast?.error('加载失败', '无法获取订阅源列表')
  } finally {
    loading.value = false
  }
}

async function saveSource() {
  if (!form.value.name) {
    window.$toast?.warning('表单不完整', '名称必填')
    return
  }
  if (form.value.content_mode === 'url' && !form.value.url) {
    window.$toast?.warning('表单不完整', '订阅链接必填')
    return
  }
  if (form.value.content_mode === 'local' && !form.value.local_nodes) {
    window.$toast?.warning('表单不完整', '本地节点内容必填')
    return
  }

  const payload = {
    name: form.value.name,
    type: form.value.content_mode === 'local' ? 'local' : form.value.type,
    url: form.value.content_mode === 'url' ? form.value.url : '',
    local_nodes: form.value.content_mode === 'local' ? form.value.local_nodes : ''
  }

  saving.value = true
  try {
    if (isEdit.value) {
      await api.updateSource(editingId.value, payload)
      window.$toast?.success('修改成功', '订阅源已更新')
    } else {
      await api.createSource(payload)
      window.$toast?.success('添加成功', '订阅源已保存')
    }
    showForm.value = false
    await load()
  } catch (e) {
    window.$toast?.error('保存失败', e.message)
  } finally {
    saving.value = false
  }
}

async function deleteSource(s) {
  if (!confirm(`确认删除源 "${s.name}"？`)) return
  try {
    await api.deleteSource(s.id)
    window.$toast?.success('已删除', '订阅源已移除')
    await load()
  } catch (e) {
    window.$toast?.error('删除失败', e.message)
  }
}

async function testSource(s) {
  testingId.value = s.id
  try {
    testData.value = await api.testAndSaveSource(s.id)
    showTestResult.value = true
    // refresh list to update last_fetch_status visually
    load()
  } catch (e) {
    window.$toast?.error('测试失败', e.message)
  } finally {
    testingId.value = null
  }
}
</script>
