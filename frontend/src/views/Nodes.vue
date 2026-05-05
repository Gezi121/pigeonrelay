<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">节点管理</h1>
        <p class="text-sm text-slate-500 mt-1">手动管理代理节点，支持批量粘贴导入与连通性测试。</p>
      </div>
      <div class="flex gap-2">
        <Button variant="secondary" icon="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" @click="openImport">批量导入</Button>
        <Button icon="M12 4v16m8-8H4" @click="openForm">添加节点</Button>
      </div>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading" class="space-y-4">
      <Card v-for="i in 3" :key="i" class="animate-pulse">
        <div class="h-5 bg-slate-200 rounded w-1/4 mb-3"></div>
        <div class="h-4 bg-slate-100 rounded w-1/2 mb-2"></div>
        <div class="h-4 bg-slate-100 rounded w-1/6"></div>
      </Card>
    </div>

    <!-- 空态 -->
    <Card v-else-if="nodes.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
        </svg>
      </div>
      <p class="text-slate-600 font-medium mb-1">暂无节点</p>
      <p class="text-sm text-slate-400 mb-6">点击右上角按钮添加或批量导入</p>
      <div class="flex gap-2">
        <Button variant="secondary" @click="openImport">批量导入</Button>
        <Button @click="openForm">添加节点</Button>
      </div>
    </Card>

    <!-- 批量操作栏 -->
    <div v-if="selectedIds.size > 0" class="sticky top-0 z-10 bg-white border border-slate-200 rounded-xl p-3 mb-4 flex items-center justify-between shadow-lg">
      <span class="text-sm text-slate-600">已选 <strong>{{ selectedIds.size }}</strong> 个节点</span>
      <div class="flex gap-2">
        <Button size="sm" variant="primary" @click="showBatchAddNode = true">批量添加优选</Button>
        <Button size="sm" variant="ghost" @click="selectedIds.clear()">取消选择</Button>
      </div>
    </div>

    <!-- 批量添加优选弹窗 -->
    <Modal v-model="showBatchAddNode" title="批量添加优选 - {{ selectedIds.size }} 个节点">
      <div class="space-y-3">
        <div class="flex gap-2">
          <select v-model="batchNodeSource" class="input-glass text-xs py-1.5 w-20">
            <option value="local">本地</option>
            <option value="external">外部</option>
          </select>
          <Input v-model="batchNodePrefix" placeholder="名称前缀" class="flex-1" />
        </div>
        <textarea v-model="batchNodeDomains" rows="5" class="input-glass text-xs w-full" placeholder="粘贴域名，一行一个"></textarea>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showBatchAddNode = false">取消</Button>
        <Button variant="primary" @click="doBatchAddToNodes" :loading="batchAdding" :disabled="!batchNodeDomains.trim()">添加到 {{ selectedIds.size }} 个节点</Button>
      </template>
    </Modal>

    <!-- 列表 -->
    <TransitionGroup v-if="nodes.length > 0" name="list" tag="div" class="grid gap-4">
      <Card v-for="(n, idx) in nodes" :key="n.id" class="group relative overflow-hidden">
        <div class="absolute left-0 top-0 bottom-0 w-1" :class="n.status === 'ok' ? 'bg-success' : (n.status ? 'bg-danger' : 'bg-warning')"></div>
        <div class="flex items-start justify-between">
          <div class="flex-1 min-w-0 pr-4">
            <div class="flex items-center gap-3 mb-1 min-w-0 flex-nowrap">
              <input type="checkbox" :checked="selectedIds.has(n.id)" @change="toggleSelect(n.id)" class="w-4 h-4 rounded border-slate-300 text-primary-600 focus:ring-primary-500 opacity-60 group-hover:opacity-100 shrink-0" />
              <h3 class="font-bold text-slate-800 truncate">{{ n.name }}</h3>
              <Badge :variant="protocolVariant(n.protocol)" class="shrink-0">{{ n.protocol?.toUpperCase() }}</Badge>
              <Badge :dot="true" :variant="n.status === 'ok' ? 'success' : (n.status ? 'danger' : 'warning')" class="shrink-0">
                {{ n.status === 'ok' ? '已通' : (n.status ? '异常' : '未测') }}
              </Badge>
              <Badge v-if="n.enabled === false" variant="danger" class="shrink-0">已禁用</Badge>
              <Badge v-if="n.local_opt_domain" :dot="true" variant="primary" class="shrink-0">优选</Badge>
            </div>
            <div class="flex items-center gap-2 text-sm text-slate-500 mt-2">
              <span class="font-mono">{{ n.address }}:{{ n.port }}</span>
              <span v-if="n.sni" class="text-xs bg-slate-100 px-1.5 py-0.5 rounded">SNI:{{ n.sni }}</span>
              <span v-if="n.last_tested_at" class="text-xs text-slate-400">[{{ fmtDate(n.last_tested_at) }}]</span>
            </div>
          </div>
          <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button size="sm" variant="secondary" icon="M5 15l7-7 7 7" @click="moveUp(idx)" :disabled="idx === 0" title="上移">↑</Button>
            <Button size="sm" variant="secondary" icon="M19 9l-7 7-7-7" @click="moveDown(idx)" :disabled="idx === nodes.length - 1" title="下移">↓</Button>
            <Button size="sm" :variant="n.enabled === false ? 'danger' : 'secondary'" icon="M3.98 8.223A10.477 10.477 0 001.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88" @click="toggleEnabled(n)" :title="n.enabled === false ? '已禁用' : '点击禁用'">{{ n.enabled === false ? '⊘' : '👁' }}</Button>
            <Button size="sm" variant="secondary" icon="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z" @click="openSettings(n)">设置</Button>
            <Button size="sm" variant="secondary" icon="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" @click="editNode(n)">编辑</Button>
            <Button size="sm" variant="secondary" icon="M13 10V3L4 14h7v7l9-11h-7z" @click="doTest(n)" :loading="testingId === n.id">测试</Button>
            <Button size="sm" variant="danger" icon="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" @click="doDelete(n)">删除</Button>
          </div>
        </div>
      </Card>
    </TransitionGroup>

    <!-- 新建/编辑弹窗 -->
    <Modal v-model="showForm" :title="isEdit ? '编辑节点' : '添加节点'">
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1">名称</label>
            <Input v-model="form.name" placeholder="节点名称" />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1">协议</label>
            <select v-model="form.protocol" class="input-glass">
              <option value="vless">VLESS</option>
              <option value="vmess">VMess</option>
              <option value="trojan">Trojan</option>
              <option value="ss">Shadowsocks</option>
              <option value="hysteria2">Hysteria2</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-3 gap-4">
          <div class="col-span-2">
            <label class="block text-sm font-medium text-slate-700 mb-1">地址</label>
            <Input v-model="form.address" placeholder="1.2.3.4" />
          </div>
          <div>
            <label class="block text-sm font-medium text-slate-700 mb-1">端口</label>
            <Input v-model.number="form.port" type="number" placeholder="443" />
          </div>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">原始链接</label>
          <Input v-model="form.raw_url" placeholder="vless://..." />
        </div>

        <!-- TLS / 传输折叠区 -->
        <details class="bg-slate-50 rounded-xl p-4 border border-slate-100">
          <summary class="text-sm font-medium text-slate-600 cursor-pointer select-none">TLS & 传输参数</summary>
          <div class="grid grid-cols-2 gap-4 mt-4">
            <div><label class="block text-xs text-slate-500 mb-1">SNI</label><Input v-model="form.sni" placeholder="example.com" /></div>
            <div><label class="block text-xs text-slate-500 mb-1">Host</label><Input v-model="form.host" placeholder="example.com" /></div>
            <div><label class="block text-xs text-slate-500 mb-1">Path</label><Input v-model="form.path" placeholder="/ws" /></div>
            <div><label class="block text-xs text-slate-500 mb-1">Security</label>
              <select v-model="form.security" class="input-glass text-sm"><option value="">无</option><option value="tls">TLS</option><option value="reality">Reality</option></select>
            </div>
            <div><label class="block text-xs text-slate-500 mb-1">ALPN</label><Input v-model="form.alpn" placeholder="h2,http/1.1" /></div>
            <div><label class="block text-xs text-slate-500 mb-1">Fingerprint</label><Input v-model="form.fingerprint" placeholder="chrome" /></div>
            <div><label class="block text-xs text-slate-500 mb-1">Transport</label>
              <select v-model="form.transport" class="input-glass text-sm"><option value="tcp">TCP</option><option value="ws">WebSocket</option><option value="grpc">gRPC</option><option value="h2">HTTP/2</option></select>
            </div>
            <div><label class="block text-xs text-slate-500 mb-1">Encryption</label><Input v-model="form.encryption" placeholder="none" /></div>
          </div>
        </details>

        <!-- 高级设置区 -->
        <details class="bg-slate-50 rounded-xl p-4 border border-slate-100">
          <summary class="text-sm font-medium text-slate-600 cursor-pointer select-none">高级设置</summary>
          <div class="grid grid-cols-2 gap-4 mt-4">
            <div>
              <label class="block text-xs text-slate-500 mb-1">名称前缀</label>
              <Input v-model="form.name_prefix" placeholder="🇭🇰 香港-" />
            </div>
            <div>
              <label class="block text-xs text-slate-500 mb-1">本地优选绑定</label>
              <select v-model="form.local_opt_domain" class="input-glass text-sm">
                <option value="">不绑定</option>
                <option v-for="dm in domainMappings" :key="dm.id" :value="dm.full_domain">{{ dm.subdomain }}</option>
              </select>
            </div>
          </div>
        </details>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showForm = false">取消</Button>
        <Button variant="primary" @click="saveNode" :loading="saving">保存</Button>
      </template>
    </Modal>

    <!-- 批量导入弹窗 -->
    <Modal v-model="showImport" title="批量导入节点">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">粘贴分享链接（每行一个）</label>
          <textarea v-model="importText" rows="6" class="input-glass font-mono text-xs" placeholder="vless://uuid@host:port?security=tls&sni=xxx&type=ws#名称&#10;trojan://pass@host:443?sni=xxx#名称"></textarea>
        </div>
        <Button variant="secondary" icon="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l1 1v6.5" @click="doParse" :loading="parsing" class="w-full">解析链接</Button>

        <!-- 解析预览 -->
        <div v-if="parseResults.length" class="max-h-64 overflow-y-auto rounded-xl border border-slate-100 bg-white">
          <table class="w-full text-xs text-left">
            <thead class="bg-slate-50 sticky top-0 border-b border-slate-100">
              <tr><th class="px-3 py-2 font-medium text-slate-500">名称</th><th class="px-3 py-2 font-medium text-slate-500">协议</th><th class="px-3 py-2 font-medium text-slate-500">地址:端口</th><th class="px-3 py-2 font-medium text-slate-500">SNI</th></tr>
            </thead>
            <tbody class="divide-y divide-slate-100">
              <tr v-for="(p, i) in parseResults" :key="i" class="hover:bg-slate-50/50">
                <td class="px-3 py-2 font-medium text-slate-700 truncate max-w-[130px]">{{ p.name }}</td>
                <td class="px-3 py-2"><Badge :variant="protocolVariant(p.protocol)">{{ p.protocol }}</Badge></td>
                <td class="px-3 py-2 text-slate-500 font-mono">{{ p.address }}:{{ p.port }}</td>
                <td class="px-3 py-2 text-slate-400">{{ p.sni || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showImport = false">取消</Button>
        <Button variant="primary" @click="doBatchCreate" :loading="saving" :disabled="!parseResults.length">导入 {{ parseResults.length }} 个节点</Button>
      </template>
    </Modal>

    <!-- 测试结果弹窗 -->
    <Modal v-model="showTestResult" :title="'测试: ' + testNodeName">
      <div class="text-center py-6">
        <div v-if="testOk" class="text-success">
          <svg class="w-12 h-12 mx-auto mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          <p class="text-lg font-bold">连通正常</p>
        </div>
        <div v-else class="text-danger">
          <svg class="w-12 h-12 mx-auto mb-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
          <p class="text-lg font-bold">连接失败</p>
        </div>
      </div>
      <template #footer>
        <Button variant="primary" @click="showTestResult = false">关闭</Button>
      </template>
    </Modal>

    <!-- 高级设置弹窗 -->
    <Modal v-model="showSettings" title="节点高级设置">
      <div v-if="settingsNode" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">名称前缀</label>
          <Input v-model="settingsForm.name_prefix" placeholder="🇭🇰 香港-" />
        </div>

        <!-- CF优选列表 -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm font-medium text-slate-700">CF 优选绑定</label>
            <Button size="sm" variant="secondary" icon="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3" @click="copyCFBindings" title="复制当前CF设置">复制</Button>
              <Button size="sm" variant="secondary" icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" @click="pasteCFBindings" title="粘贴CF设置">粘贴</Button>
              <Button size="sm" variant="secondary" icon="M12 4v16m8-8H4" @click="addCFBinding">添加</Button>
              <Button size="sm" variant="secondary" @click="showBatchAdd = !showBatchAdd">批量添加</Button>
          </div>
          <div v-if="showBatchAdd" class="bg-slate-50 rounded-lg p-3 space-y-2 border border-slate-200 mb-2">
            <div class="flex gap-2">
              <select v-model="batchSource" class="input-glass text-xs py-1.5 w-20">
                <option value="local">本地</option>
                <option value="external">外部</option>
              </select>
              <Input v-model="batchPrefix" placeholder="名称前缀" class="flex-1" />
            </div>
            <textarea v-model="batchDomains" rows="4" class="input-glass text-xs w-full" placeholder="粘贴域名，一行一个&#10;例如：&#10;up.cdn.gugugezi.com&#10;down.cdn.gugugezi.com&#10;cdn.gugugezi.com"></textarea>
            <div class="flex gap-2">
              <Button size="sm" variant="primary" @click="doBatchAdd" :disabled="!batchDomains.trim()">确认添加</Button>
              <Button size="sm" variant="ghost" @click="showBatchAdd = false">取消</Button>
            </div>
          </div>
          <div v-if="cfBindings.length === 0 && !showBatchAdd" class="text-xs text-slate-400 bg-slate-50 rounded-lg p-3 text-center">
            暂无优选绑定
          </div>
          <div v-else class="space-y-2">
            <div v-for="(b, i) in cfBindings" :key="i" class="flex items-center gap-2 bg-slate-50 rounded-lg p-2 border border-slate-100">
              <span class="text-xs text-slate-400 w-5">{{ i + 1 }}</span>
              <select v-model="b.source" @change="b.full_domain = ''" class="input-glass text-xs py-1.5 w-20">
                <option value="local">本地</option>
                <option value="external">外部</option>
              </select>
              <select v-model="b.full_domain" class="input-glass text-xs flex-1 py-1.5">
                <option value="">选择域名</option>
                <option v-for="dm in filteredDomains(b.source)" :key="dm.id" :value="dm.full_domain">{{ dm.subdomain }}</option>
              </select>
              <Input v-model="b.name_prefix" placeholder="前缀 如: 优选1-" class="flex-1" />
              <button @click="cfBindings.splice(i, 1)" class="text-slate-400 hover:text-danger transition-colors shrink-0">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
              </button>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <label class="text-sm font-medium text-slate-700">启用</label>
          <input type="checkbox" v-model="settingsForm.enabled" class="w-5 h-5 rounded border-slate-300 text-primary-600 focus:ring-primary-500" />
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showSettings = false">取消</Button>
        <Button variant="primary" @click="saveSettings" :loading="savingSettings">保存设置</Button>
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

const nodes = ref([])
const domainMappings = ref([])
const loading = ref(true)
const saving = ref(false)
const savingSettings = ref(false)
const showForm = ref(false)
const showImport = ref(false)
const showSettings = ref(false)
const showTestResult = ref(false)

const isEdit = ref(false)
const editingId = ref(null)
const form = ref({ name: '', protocol: 'vless', address: '', port: 443, raw_url: '', sni: '', host: '', path: '', security: '', alpn: '', fingerprint: '', transport: 'tcp', encryption: '', name_prefix: '', local_opt_domain: '' })

const importText = ref('')
const parseResults = ref([])
const parsing = ref(false)

const testingId = ref(null)
const testOk = ref(false)
const testNodeName = ref('')

const settingsNode = ref(null)
const settingsForm = ref({ name_prefix: '', local_opt_domain: '', enabled: true })

const PROTOTYPES = { vless: 'primary', vmess: 'warning', trojan: 'info', ss: 'success', hysteria2: 'accent' }
function protocolVariant(p) { return PROTOTYPES[p] || 'primary' }
function fmtDate(d) { return new Date(d).toLocaleDateString([], { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }) }

onMounted(load)

async function load() {
  try {
    const [n, dm] = await Promise.all([api.listNodes(), api.listDomainMappings().catch(() => [])])
    nodes.value = n || []
    domainMappings.value = dm || []
  } catch {
    window.$toast?.error('加载失败', '无法获取节点列表')
  } finally {
    loading.value = false
  }
}

async function moveUp(idx) {
  if (idx <= 0) return
  const list = [...nodes.value]
  ;[list[idx - 1], list[idx]] = [list[idx], list[idx - 1]]
  await applyOrder(list)
}

async function moveDown(idx) {
  if (idx >= nodes.value.length - 1) return
  const list = [...nodes.value]
  ;[list[idx], list[idx + 1]] = [list[idx + 1], list[idx]]
  await applyOrder(list)
}

async function applyOrder(list) {
  nodes.value = list
  try {
    await api.reorderNodes(list.map(n => n.id))
  } catch {
    window.$toast?.error('排序失败', '无法更新排序')
  }
}

async function toggleEnabled(n) {
  const newVal = n.enabled === false ? true : false
  try {
    await api.updateNode(n.id, { enabled: newVal })
    n.enabled = newVal
  } catch {
    window.$toast?.error('操作失败', '无法更新状态')
  }
}

function resetForm() {
  form.value = { name: '', protocol: 'vless', address: '', port: 443, raw_url: '', sni: '', host: '', path: '', security: '', alpn: '', fingerprint: '', transport: 'tcp', encryption: '', name_prefix: '', local_opt_domain: '' }
}

function openForm() { isEdit.value = false; editingId.value = null; resetForm(); showForm.value = true }
function editNode(n) {
  isEdit.value = true; editingId.value = n.id
  form.value = {
    name: n.name || '', protocol: n.protocol || 'vless', address: n.address || '', port: n.port || 443,
    raw_url: n.raw_url || '', sni: n.sni || '', host: n.host || '', path: n.path || '',
    security: n.security || '', alpn: n.alpn || '', fingerprint: n.fingerprint || '',
    transport: n.transport || 'tcp', encryption: n.encryption || '',
    name_prefix: n.name_prefix || '', local_opt_domain: n.local_opt_domain || ''
  }
  showForm.value = true
}

async function saveNode() {
  if (!form.value.name || !form.value.address) { window.$toast?.warning('表单不完整', '名称和地址必填'); return }
  saving.value = true
  try {
    if (isEdit.value) {
      await api.updateNode(editingId.value, form.value)
      window.$toast?.success('已更新', '节点信息已保存')
    } else {
      await api.createNode(form.value)
      window.$toast?.success('创建成功', '节点已添加')
    }
    showForm.value = false
    await load()
  } catch (e) {
    window.$toast?.error('保存失败', e.message)
  } finally {
    saving.value = false
  }
}

function openImport() { importText.value = ''; parseResults.value = []; showImport.value = true }

async function doParse() {
  if (!importText.value.trim()) { window.$toast?.warning('输入为空', '请粘贴分享链接'); return }
  parsing.value = true
  try {
    const res = await api.parseNodeLink(importText.value)
    parseResults.value = res.nodes || []
    if (!parseResults.value.length) window.$toast?.warning('解析结果为空', '未识别有效链接')
  } catch (e) {
    window.$toast?.error('解析失败', e.message)
  } finally {
    parsing.value = false
  }
}

async function doBatchCreate() {
  saving.value = true
  let created = 0
  try {
    for (const p of parseResults.value) {
      try {
        await api.createNode({
          name: p.name, protocol: p.protocol, address: p.address, port: p.port,
          raw_url: p.raw_url, sni: p.sni, host: p.host, path: p.path,
          security: p.security, alpn: p.alpn, fingerprint: p.fingerprint,
          transport: p.transport, encryption: p.encryption
        })
        created++
      } catch { /* skip duplicates */ }
    }
    window.$toast?.success('导入完成', `成功创建 ${created}/${parseResults.value.length} 个节点`)
    showImport.value = false
    await load()
  } finally {
    saving.value = false
  }
}

async function doTest(n) {
  testingId.value = n.id
  try {
    const res = await api.testNode(n.id)
    testOk.value = res.ok
    testNodeName.value = n.name
    showTestResult.value = true
    await load()
  } catch {
    window.$toast?.error('测试失败', '连通性检测异常')
  } finally {
    testingId.value = null
  }
}

async function doDelete(n) {
  if (!confirm(`确认删除节点 "${n.name}"？`)) return
  try {
    await api.deleteNode(n.id)
    window.$toast?.success('已删除', `节点 ${n.name} 已移除`)
    await load()
  } catch (e) {
    window.$toast?.error('删除失败', e.message)
  }
}

const cfBindings = ref([])
const showBatchAdd = ref(false)
const batchSource = ref('external')
const batchPrefix = ref('')
const batchDomains = ref('')

const selectedIds = ref(new Set())
const showBatchAddNode = ref(false)
const batchNodeSource = ref('external')
const batchNodePrefix = ref('')
const batchNodeDomains = ref('')
const batchAdding = ref(false)

function toggleSelect(id) {
  const s = selectedIds.value
  if (s.has(id)) { s.delete(id) } else { s.add(id) }
  selectedIds.value = new Set(s)
}

function addCFBinding() {
  cfBindings.value.push({ full_domain: '', name_prefix: '', source: 'local' })
}

function doBatchAdd() {
  const domains = batchDomains.value.split('\n').map(d => d.trim()).filter(Boolean)
  if (!domains.length) return
  const prefix = batchPrefix.value
  const source = batchSource.value
  for (const domain of domains) {
    cfBindings.value.push({ full_domain: domain, name_prefix: prefix, source })
  }
  batchDomains.value = ''
  batchPrefix.value = ''
  showBatchAdd.value = false
}

async function doBatchAddToNodes() {
  const domains = batchNodeDomains.value.split('\n').map(d => d.trim()).filter(Boolean)
  if (!domains.length) return
  batchAdding.value = true
  let ok = 0, fail = 0
  for (const id of selectedIds.value) {
    try {
      for (const domain of domains) {
        await api.createCFBinding(id, { full_domain: domain, name_prefix: batchNodePrefix.value, source: batchNodeSource.value })
      }
      ok++
    } catch { fail++ }
  }
  batchAdding.value = false
  showBatchAddNode.value = false
  batchNodeDomains.value = ''
  selectedIds.value.clear()
  window.$toast?.success(`完成: ${ok} 个节点成功${fail ? ', ' + fail + ' 个失败' : ''}`)
}

async function copyCFBindings() {
  const valid = cfBindings.value.filter(b => b.full_domain)
  if (!valid.length) { window.$toast?.warning('无CF设置', '请先添加优选域名绑定'); return }
  const data = JSON.stringify(valid.map(b => ({ full_domain: b.full_domain, name_prefix: b.name_prefix || '', source: b.source || 'local' })))
  try {
    await navigator.clipboard.writeText(data)
    window.$toast?.success('已复制', `已复制 ${valid.length} 条CF优选设置`)
  } catch {
    window.$toast?.error('复制失败', '请手动复制')
  }
}

async function pasteCFBindings() {
  try {
    const text = await navigator.clipboard.readText()
    const data = JSON.parse(text)
    if (!Array.isArray(data) || !data.length) throw new Error('无效格式')
    for (const item of data) {
      if (item.full_domain) {
        cfBindings.value.push({ full_domain: item.full_domain, name_prefix: item.name_prefix || '', source: item.source || 'local' })
      }
    }
    window.$toast?.success('已粘贴', `粘贴 ${data.length} 条CF优选设置`)
  } catch {
    window.$toast?.error('粘贴失败', '剪贴板无有效CF设置数据')
  }
}

function filteredDomains(source) {
  if (source === 'external') return domainMappings.value.filter(d => d.source === 'external')
  return domainMappings.value.filter(d => d.source !== 'external')
}

async function openSettings(n) {
  settingsNode.value = n
  settingsForm.value = { name_prefix: n.name_prefix || '', enabled: n.enabled !== false }
  // Load existing CF bindings
  try {
    cfBindings.value = (await api.listCFBindings(n.id)) || []
  } catch {
    cfBindings.value = []
  }
  showSettings.value = true
}

async function saveSettings() {
  savingSettings.value = true
  try {
    await api.updateNode(settingsNode.value.id, settingsForm.value)
    // Sync CF bindings: delete all, recreate
    const existing = await api.listCFBindings(settingsNode.value.id).catch(() => [])
    for (const b of existing) {
      await api.deleteCFBinding(settingsNode.value.id, b.id).catch(() => {})
    }
    for (const b of cfBindings.value) {
      if (b.full_domain) {
        await api.createCFBinding(settingsNode.value.id, { full_domain: b.full_domain, name_prefix: b.name_prefix, source: b.source || 'local' }).catch(() => {})
      }
    }
    window.$toast?.success('已保存', '高级设置已更新')
    showSettings.value = false
    await load()
  } catch (e) {
    window.$toast?.error('保存失败', e.message)
  } finally {
    savingSettings.value = false
  }
}
</script>
