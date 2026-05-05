<template>
  <div class="max-w-4xl mx-auto animate-fade-in">
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-slate-800 tracking-tight">测速客户端</h1>
      <p class="text-sm text-slate-500 mt-1">查看各测速客户端设备的今日测速记录，设置备注名称。</p>
    </div>

    <div v-if="loading" class="space-y-4">
      <Card v-for="i in 3" :key="i" class="animate-pulse"><div class="h-5 bg-slate-200 rounded w-1/3 mb-2"></div><div class="h-4 bg-slate-100 rounded w-1/2"></div></Card>
    </div>

    <Card v-else-if="clients.length === 0" class="text-center py-16 text-slate-400">
      暂无测速客户端上报数据 — 部署测速脚本后 client_id 会自动出现在这里
    </Card>

    <TransitionGroup v-else name="list" tag="div" class="space-y-3">
      <Card v-for="c in clients" :key="c.client_id" class="group">
        <div class="flex items-center gap-4">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="font-bold text-slate-800 font-mono text-sm">{{ c.client_id }}</span>
              <Badge v-if="c.name" variant="primary">{{ c.name }}</Badge>
              <Badge :dot="true" variant="success">今日 {{ c.today_count }} 条</Badge>
            </div>
            <p v-if="c.notes" class="text-xs text-slate-400 mt-1">{{ c.notes }}</p>
          </div>
          <div class="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
            <Button size="sm" variant="secondary" @click="openEdit(c)">备注</Button>
            <Button size="sm" variant="danger" @click="confirmClear(c)">清空记录</Button>
          </div>
        </div>
      </Card>
    </TransitionGroup>

    <Modal v-model="showEdit" title="编辑客户端备注">
      <div v-if="editItem" class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">Client ID</label>
          <p class="font-mono text-xs text-slate-600 truncate">{{ editItem.client_id }}</p>
        </div>
        <div><label class="block text-sm font-medium text-slate-700 mb-1">名称</label><Input v-model="editForm.name" placeholder="如: 北京联通-01" /></div>
        <div><label class="block text-sm font-medium text-slate-700 mb-1">备注</label><Input v-model="editForm.notes" placeholder="如: 联通千兆宽带" /></div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="showEdit = false">取消</Button>
        <Button variant="primary" @click="saveEdit" :loading="saving">保存</Button>
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

const clients = ref([])
const loading = ref(true)
const saving = ref(false)
const showEdit = ref(false)
const editItem = ref(null)
const editForm = ref({ name: '', notes: '' })

onMounted(load)

async function load() {
  try { clients.value = (await api.listSpeedClients()) || [] }
  catch { window.$toast?.error('加载失败') }
  finally { loading.value = false }
}

function openEdit(c) {
  editItem.value = c
  editForm.value = { name: c.name || '', notes: c.notes || '' }
  showEdit.value = true
}

async function confirmClear(c) {
  if (!confirm(`⚠️ 确认清空客户端 ${c.name || c.client_id} 的全部历史测速记录？\n\n此操作不可撤销。`)) return
  try {
    const res = await api.deleteClientRecords(c.client_id)
    window.$toast?.success('已清空', `删除了 ${res.count} 条记录`)
    await load()
  } catch (e) { window.$toast?.error('操作失败', e.message) }
}

async function saveEdit() {
  saving.value = true
  try {
    await api.upsertSpeedClient({ client_id: editItem.value.client_id, ...editForm.value })
    window.$toast?.success('已保存')
    showEdit.value = false
    await load()
  } catch (e) { window.$toast?.error('保存失败', e.message) }
  finally { saving.value = false }
}
</script>
