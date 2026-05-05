<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">节点分组</h1>
        <p class="text-sm text-slate-500 mt-1">创建分组以统一配置、规则并生成新的订阅配置。</p>
      </div>
      <Button icon="M12 4v16m8-8H4" @click="showForm = true">新建分组</Button>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading" class="grid gap-4 md:grid-cols-2">
      <Card v-for="i in 4" :key="i" class="animate-pulse h-32">
        <div class="h-5 bg-slate-200 rounded w-1/3 mb-3"></div>
        <div class="h-4 bg-slate-100 rounded w-1/4"></div>
      </Card>
    </div>

    <!-- 空态 -->
    <Card v-else-if="groups.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
      </div>
      <p class="text-slate-600 font-medium mb-1">暂无节点分组</p>
      <p class="text-sm text-slate-400 mb-6">点击右上角按钮创建您的第一个分组</p>
      <Button variant="secondary" icon="M12 4v16m8-8H4" @click="showForm = true">新建分组</Button>
    </Card>

    <!-- 数据列表 -->
    <TransitionGroup v-else name="list" tag="div" class="grid gap-5 md:grid-cols-2">
      <Card v-for="g in groups" :key="g.id" class="group hover:-translate-y-1 transition-all duration-300">
        <div class="flex items-start justify-between mb-4">
          <div>
            <h3 class="text-lg font-bold text-slate-800">{{ g.name }}</h3>
            <p class="text-xs text-slate-500 mt-1 flex items-center gap-1.5">
              <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
              </svg>
              <span>{{ getSourceName(g.source_id) || '手动来源' }}</span>
            </p>
          </div>
          <Badge variant="primary">ID: {{ g.id }}</Badge>
        </div>

        <div class="flex gap-2 mt-6">
          <Button class="flex-1" variant="secondary" @click="editGroup(g)">高级配置</Button>
          <Button class="flex-1" variant="primary" @click="generate(g.id)" :loading="generatingId === g.id">一键生成</Button>
        </div>
      </Card>
    </TransitionGroup>

    <!-- 新建弹窗 -->
    <Modal v-model="showForm" title="创建新分组">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">分组名称</label>
          <Input v-model="form.name" placeholder="例如：优化节点组" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">绑定的订阅源</label>
          <select v-model="form.source_id" class="input-glass">
            <option :value="null">— 选择已有订阅源 —</option>
            <option v-for="s in sources" :key="s.id" :value="s.id">{{ s.name }} ({{ s.type }})</option>
          </select>
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showForm = false">取消</Button>
        <Button variant="primary" @click="createGroup" :loading="saving">创建分组</Button>
      </template>
    </Modal>

    <!-- 高级配置弹窗 -->
    <Modal v-model="showConfig" :title="`配置: ${editingGroup?.name}`">
      <div class="space-y-5">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">节点名称前缀</label>
          <p class="text-xs text-slate-500 mb-2">生成节点时自动添加到每个节点名称前。</p>
          <Input v-model="editForm.name_prefix" placeholder="例如: [VIP] " />
        </div>

        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">分组类型</label>
          <select v-model="editForm.group_type" class="input-glass">
            <option value="standard">普通节点组</option>
            <option value="local_opt">本地优选组</option>
          </select>
        </div>

        <div class="mt-4">
          <div class="flex justify-between items-center mb-2">
            <div>
              <label class="block text-sm font-medium text-slate-700">端口/SNI替换规则</label>
              <p class="text-xs text-slate-500">将组内节点强制替换端口或SNI配置。</p>
            </div>
            <Button size="sm" @click="addReplacement">+ 添加规则</Button>
          </div>
          <div v-for="(rule, i) in editForm.replacements" :key="i" class="flex gap-2 mb-2 items-center">
            <select v-model="rule.target_field" class="input-glass w-1/3">
              <option value="port">替换端口</option>
              <option value="sni">替换SNI</option>
            </select>
            <input v-model="rule.replace_value" type="text" class="input-glass flex-1" placeholder="替换值 (如 443 或 example.com)" />
            <Button variant="danger" size="sm" @click="removeReplacement(i)">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </Button>
          </div>
        </div>

        <div class="mt-4 border-t border-slate-100 pt-4">
          <div class="flex justify-between items-center mb-2">
            <div>
              <label class="block text-sm font-medium text-slate-700">高级优选地址</label>
              <p class="text-xs text-slate-500">为每个原始节点复制并应用多组优选 IP/域名。</p>
            </div>
            <Button size="sm" @click="addRouting">+ 添加地址</Button>
          </div>
          <div v-for="(route, i) in editForm.routings" :key="'r'+i" class="flex gap-2 mb-2 items-center">
            <input v-model="route.routing_address" type="text" class="input-glass flex-1" placeholder="IP地址或域名 (如 1.1.1.1)" />
            <Button variant="danger" size="sm" @click="removeRouting(i)">
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </Button>
          </div>
        </div>

      </div>
      <template #footer>
        <Button variant="ghost" @click="showConfig = false">取消</Button>
        <Button variant="primary" @click="saveConfig" :loading="saving">保存配置</Button>
      </template>
    </Modal>

    <!-- 生成结果弹窗 -->
    <Modal v-model="showResult" title="配置生成成功">
      <div class="space-y-4">
        <p class="text-sm text-slate-600">已根据分组规则生成最新配置，内容已自动备份。</p>
        <div class="relative group">
          <pre class="text-xs text-slate-600 max-h-96 overflow-auto bg-slate-50 border border-slate-100 p-4 rounded-xl font-mono whitespace-pre-wrap break-all">{{ genResult }}</pre>
          <Button
            size="sm"
            variant="primary"
            class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity"
            @click="copyConfig"
          >
            复制配置
          </Button>
        </div>
      </div>
      <template #footer>
        <Button variant="primary" @click="showResult = false">关闭</Button>
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

const groups = ref([])
const sources = ref([])
const loading = ref(true)

const showForm = ref(false)
const saving = ref(false)
const form = ref({ name: '', source_type: 'subscription', source_id: null })

const showConfig = ref(false)
const editingGroup = ref(null)
const editForm = ref({ name_prefix: '', group_type: 'standard', replacements: [], routings: [] })

function addReplacement() {
  editForm.value.replacements.push({ target_field: 'port', replace_value: '' })
}

function removeReplacement(index) {
  editForm.value.replacements.splice(index, 1)
}

function addRouting() {
  editForm.value.routings.push({ routing_address: '' })
}

function removeRouting(index) {
  editForm.value.routings.splice(index, 1)
}

const showResult = ref(false)
const genResult = ref('')
const generatingId = ref(null)

onMounted(load)

async function load() {
  loading.value = true
  try {
    [groups.value, sources.value] = await Promise.all([api.listGroups(), api.listSources()])
  } catch {
    window.$toast?.error('加载失败', '无法获取数据')
  } finally {
    loading.value = false
  }
}

function getSourceName(id) {
  if (!id) return ''
  const s = sources.value.find(s => s.id === id)
  return s ? s.name : `ID: ${id}`
}

async function createGroup() {
  if (!form.value.name) {
    window.$toast?.warning('名称为空', '请输入分组名称')
    return
  }

  saving.value = true
  try {
    await api.createGroup({
      name: form.value.name,
      source_type: 'subscription',
      source_id: form.value.source_id,
    })
    form.value = { name: '', source_type: 'subscription', source_id: null }
    showForm.value = false
    window.$toast?.success('创建成功', '分组已建立')
    await load()
  } catch (e) {
    window.$toast?.error('创建失败', e.message)
  } finally {
    saving.value = false
  }
}

function editGroup(g) {
  editingGroup.value = g
  editForm.value = {
    name_prefix: g.name_prefix || '',
    group_type: g.group_type || 'standard',
    replacements: g.replacements ? JSON.parse(JSON.stringify(g.replacements)) : [],
    routings: g.routings ? JSON.parse(JSON.stringify(g.routings)) : []
  }
  showConfig.value = true
}

async function saveConfig() {
  saving.value = true
  try {
    await api.updateGroup(editingGroup.value.id, editForm.value)
    // 根据设计需求：自动备份在后端处理更好，前端可主动调一把备份
    await api.createBackup({ backup_type: 'clash', config_content: 'Group configuration updated.' })
    showConfig.value = false
    window.$toast?.success('保存成功', '分组高级配置已更新并备份')
    await load()
  } catch (e) {
    window.$toast?.error('保存失败', e.message)
  } finally {
    saving.value = false
  }
}

async function generate(id) {
  generatingId.value = id
  try {
    genResult.value = await api.generateConfig(id, 'clash')
    showResult.value = true
  } catch (e) {
    window.$toast?.error('生成失败', e.message)
  } finally {
    generatingId.value = null
  }
}

async function copyConfig() {
  try {
    await navigator.clipboard.writeText(genResult.value)
    window.$toast?.success('已复制', '配置内容已复制到剪贴板')
  } catch {
    const el = document.createElement('textarea')
    el.value = genResult.value
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    window.$toast?.success('已复制', '配置内容已复制到剪贴板')
  }
}
</script>
