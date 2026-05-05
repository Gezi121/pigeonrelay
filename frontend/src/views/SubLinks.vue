<template>
  <div class="max-w-4xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">订阅短链</h1>
        <p class="text-sm text-slate-500 mt-1">创建并管理 Clash / V2Ray 客户端订阅链接，支持密码保护和流量限制。</p>
      </div>
      <Button icon="M12 4v16m8-8H4" @click="showForm = true">新建短链</Button>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading" class="space-y-4">
      <Card v-for="i in 3" :key="i" class="animate-pulse"><div class="h-5 bg-slate-200 rounded w-1/3 mb-2"></div><div class="h-4 bg-slate-100 rounded w-1/2"></div></Card>
    </div>

    <!-- 空态 -->
    <Card v-else-if="links.length === 0 && !showForm" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" /></svg>
      </div>
      <p class="text-slate-600 font-medium mb-1">暂无订阅短链</p>
      <p class="text-sm text-slate-400 mb-6">点击右上角创建第一个订阅链接</p>
      <Button @click="showForm = true">新建短链</Button>
    </Card>

    <!-- 列表 -->
    <TransitionGroup v-else name="list" tag="div" class="space-y-3">
      <Card v-for="l in links" :key="l.hash" class="group">
        <div class="flex items-center gap-4">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span class="font-bold text-slate-800 truncate">{{ l.name || l.remarks || '未命名' }}</span>
              <Badge v-if="l.password_hash" variant="warning" dot>密码保护</Badge>
              <Badge :variant="l.enabled ? 'success' : 'danger'" dot>{{ l.enabled ? '启用' : '已禁用' }}</Badge>
            </div>
            <p class="text-xs font-mono text-slate-400 truncate">/sub/{{ l.hash }}</p>
            <div class="flex items-center gap-3 text-xs text-slate-500 mt-1">
              <span>流量: {{ l.traffic_limit_gb ? l.traffic_limit_gb + 'GB' : '无限' }}</span>
              <span>到期: {{ l.expire_at || '无限期' }}</span>
            </div>
          </div>
          <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button size="sm" variant="primary" @click="copyLink(l)">Clash</Button>
            <Button size="sm" variant="secondary" @click="copyLink(l, 'v2ray')">V2Ray</Button>
            <Button size="sm" variant="secondary" @click="openEdit(l)">编辑</Button>
            <Button size="sm" variant="secondary" @click="toggleEnabled(l)" :loading="togglingId === l.hash">{{ l.enabled ? '停用' : '启用' }}</Button>
            <Button size="sm" variant="danger" @click="doDelete(l)">删除</Button>
          </div>
        </div>
      </Card>
    </TransitionGroup>

    <!-- 新建弹窗 -->
    <Modal v-model="showForm" title="新建订阅短链">
      <div class="space-y-4">
        <div><label class="block text-sm font-medium text-slate-700 mb-1">订阅名称</label><Input v-model="form.name" placeholder="例如: 我的优选线路" /><p class="text-xs text-slate-400 mt-1">用作下载文件名</p></div>
        <div><label class="block text-sm font-medium text-slate-700 mb-1">访问密码（可选，留空即无密码）</label><Input v-model="form.password" placeholder="设置后客户端需 ?token=密码 访问" /></div>
        <div><label class="block text-sm font-medium text-slate-700 mb-1">备注</label><Input v-model="form.remarks" placeholder="例如: 张三的专属线路" /></div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-sm font-medium text-slate-700 mb-1">月流量限制 (GB)</label><Input v-model.number="form.traffic_limit_gb" type="number" placeholder="0 = 无限" /><p class="text-xs text-slate-400 mt-1">0 表示不限流量</p></div>
          <div><label class="block text-sm font-medium text-slate-700 mb-1">到期时间</label><Input v-model="form.expire_at" type="date" /><p class="text-xs text-slate-400 mt-1">留空表示永不过期</p></div>
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showForm = false">取消</Button>
        <Button variant="primary" @click="create" :loading="saving">立即生成</Button>
      </template>
    </Modal>

    <!-- 编辑弹窗 -->
    <Modal v-model="showEdit" title="编辑订阅短链">
      <div v-if="editItem" class="space-y-4">
        <div><label class="block text-sm font-medium text-slate-700 mb-1">订阅名称</label><Input v-model="editForm.name" /></div>
        <div><label class="block text-sm font-medium text-slate-700 mb-1">备注</label><Input v-model="editForm.remarks" /></div>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-sm font-medium text-slate-700 mb-1">月流量限制 (GB)</label><Input v-model.number="editForm.traffic_limit_gb" type="number" /></div>
          <div><label class="block text-sm font-medium text-slate-700 mb-1">到期时间</label><Input v-model="editForm.expire_at" type="date" /></div>
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showEdit = false">取消</Button>
        <Button variant="primary" @click="doEdit" :loading="saving">保存</Button>
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

const links = ref([])
const loading = ref(true)
const saving = ref(false)
const showForm = ref(false)
const togglingId = ref(null)
const showEdit = ref(false)
const editItem = ref(null)
const editForm = ref({ name: '', remarks: '', traffic_limit_gb: 0, expire_at: '' })
const form = ref({ name: '', password: '', remarks: '', traffic_limit_gb: 0, expire_at: '' })

onMounted(load)

async function load() {
  try { links.value = (await api.listSubLinks()) || [] }
  catch { window.$toast?.error('加载失败', '无法获取订阅短链列表') }
  finally { loading.value = false }
}

async function create() {
  saving.value = true
  try {
    const res = await api.createSubLink(form.value)
    const url = `${window.location.origin}/sub/${res.hash}${form.value.password ? '?token=' + form.value.password : ''}`
    await navigator.clipboard.writeText(url).catch(() => {})
    window.$toast?.success('已创建并复制', url.substring(0, 60) + '...')
    showForm.value = false
    form.value = { name: '', password: '', remarks: '', traffic_limit_gb: 0, expire_at: '' }
    await load()
  } catch (e) { window.$toast?.error('创建失败', e.message) }
  finally { saving.value = false }
}

async function copyLink(l, format) {
  const base = `${window.location.origin}/sub/${l.hash}`
  const token = l.password_hash ? '?token=你的密码' : ''
  const fmt = format === 'v2ray' ? (token ? token + '&format=v2ray' : '?format=v2ray') : token
  const url = base + fmt
  try {
    await navigator.clipboard.writeText(url)
    const label = format === 'v2ray' ? 'V2Ray' : 'Clash'
    window.$toast?.success(`已复制 ${label} 链接`, url.substring(0, 60) + '...')
  } catch {
    window.$toast?.error('复制失败')
  }
}

function openEdit(l) {
  editItem.value = l
  editForm.value = { name: l.name || '', remarks: l.remarks || '', traffic_limit_gb: l.traffic_limit_gb || 0, expire_at: l.expire_at || '' }
  showEdit.value = true
}

async function doEdit() {
  saving.value = true
  try {
    await api.updateSubLink(editItem.value.hash, editForm.value)
    window.$toast?.success('已更新', '订阅短链信息已保存')
    showEdit.value = false
    await load()
  } catch (e) { window.$toast?.error('更新失败', e.message) }
  finally { saving.value = false }
}

async function toggleEnabled(l) {
  togglingId.value = l.hash
  try {
    await api.updateSubLink(l.hash, { enabled: !l.enabled })
    l.enabled = !l.enabled
  } catch { window.$toast?.error('操作失败') }
  finally { togglingId.value = null }
}

async function doDelete(l) {
  if (!confirm(`确认删除短链 "${l.remarks || l.hash}"？`)) return
  try {
    await api.deleteSubLink(l.hash)
    window.$toast?.success('已删除')
    await load()
  } catch (e) { window.$toast?.error('删除失败', e.message) }
}
</script>
