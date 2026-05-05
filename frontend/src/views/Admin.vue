<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-slate-800 tracking-tight">用户管理</h1>
      <p class="text-sm text-slate-500 mt-1">管理员专用，用于管理系统子账号及其配额限制。</p>
    </div>

    <!-- 加载骨架屏 -->
    <Card v-if="loading" padding>
      <div class="space-y-4">
        <div v-for="i in 5" :key="i" class="h-10 bg-slate-100 rounded animate-pulse"></div>
      </div>
    </Card>

    <Card v-else padding class="overflow-hidden p-0">
      <div class="overflow-x-auto">
        <table class="w-full text-sm text-left">
          <thead class="bg-slate-50/80 border-b border-slate-100">
            <tr>
              <th class="py-3 px-4 font-semibold text-slate-600">ID</th>
              <th class="py-3 px-4 font-semibold text-slate-600">用户名</th>
              <th class="py-3 px-4 font-semibold text-slate-600">角色</th>
              <th class="py-3 px-4 font-semibold text-slate-600">到期时间</th>
              <th class="py-3 px-4 font-semibold text-slate-600">额度 / 已用</th>
              <th class="py-3 px-4 font-semibold text-slate-600 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-50">
            <tr v-for="u in users" :key="u.id" class="hover:bg-primary-50/30 transition-colors group">
              <td class="py-3 px-4 text-slate-500 font-mono">{{ u.id }}</td>
              <td class="py-3 px-4 font-medium text-slate-800">
                <div class="flex items-center gap-2">
                  <div class="w-6 h-6 rounded-full bg-gradient-to-br flex items-center justify-center text-xs text-white"
                    :class="u.role === 'admin' ? 'from-primary-500 to-primary-700' : 'from-slate-400 to-slate-500'">
                    {{ u.username.charAt(0).toUpperCase() }}
                  </div>
                  {{ u.username }}
                </div>
              </td>
              <td class="py-3 px-4">
                <Badge :variant="u.role === 'admin' ? 'primary' : 'default'" :dot="true">{{ u.role }}</Badge>
              </td>
              <td class="py-3 px-4 text-slate-600 text-xs">
                {{ u.expire_at?.slice(0, 10) || '永久有效' }}
              </td>
              <td class="py-3 px-4 text-xs">
                <div class="flex flex-col gap-1">
                  <span class="text-slate-700 font-mono">{{ formatBytes(u.used_bytes) }}</span>
                  <span class="text-slate-400 font-mono">/ {{ formatBytes(u.monthly_quota_bytes) }}</span>
                </div>
              </td>
              <td class="py-3 px-4">
                <div class="flex items-center justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Button size="sm" variant="secondary" @click="openEdit(u)">编辑</Button>
                  <Button size="sm" variant="ghost" @click="resetTraffic(u.id)" title="清零当月流量">重置</Button>
                  <Button size="sm" variant="danger" @click="deleteUser(u.id)">删除</Button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </Card>

    <!-- 编辑弹窗 -->
    <Modal v-model="editing" :title="`编辑用户: ${editingUser?.username}`">
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">月度流量额度 (Bytes)</label>
          <p class="text-xs text-slate-500 mb-2">设置 0 为无限制。1GB = 1073741824</p>
          <Input v-model.number="editForm.monthly_quota_bytes" type="number" placeholder="输入字节数" />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">账号到期时间</label>
          <p class="text-xs text-slate-500 mb-2">留空表示永久有效。</p>
          <Input v-model="editForm.expire_at" type="datetime-local" />
        </div>
      </div>
      <template #footer>
        <Button variant="ghost" @click="editing = false">取消</Button>
        <Button variant="primary" @click="saveEdit" :loading="saving">保存更改</Button>
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

const users = ref([])
const loading = ref(true)
const editing = ref(false)
const saving = ref(false)
const editingUser = ref(null)
const editForm = ref({ monthly_quota_bytes: 0, expire_at: '' })

onMounted(load)

async function load() {
  loading.value = true
  try {
    users.value = await api.adminListUsers()
  } catch (e) {
    window.$toast?.error('加载失败', e.message)
  } finally {
    loading.value = false
  }
}

function openEdit(u) {
  editingUser.value = u
  editForm.value = {
    monthly_quota_bytes: u.monthly_quota_bytes || 0,
    expire_at: u.expire_at ? u.expire_at.slice(0, 16) : '',
  }
  editing.value = true
}

async function saveEdit() {
  saving.value = true
  try {
    await api.adminUpdateUser(editingUser.value.id, editForm.value)
    editing.value = false
    window.$toast?.success('修改成功', `用户 ${editingUser.value.username} 已更新`)
    await load()
  } catch (e) {
    window.$toast?.error('保存失败', e.message)
  } finally {
    saving.value = false
  }
}

async function resetTraffic(id) {
  if (!confirm('确认清零该用户当月流量？')) return
  try {
    await api.adminResetTraffic(id)
    window.$toast?.success('重置成功', '流量已清零')
    await load()
  } catch (e) {
    window.$toast?.error('操作失败', e.message)
  }
}

async function deleteUser(id) {
  if (!confirm('危险操作：确认永久删除该用户？')) return
  try {
    await api.adminDeleteUser(id)
    window.$toast?.success('已删除', '用户数据已清除')
    await load()
  } catch (e) {
    window.$toast?.error('删除失败', e.message)
  }
}

function formatBytes(b) {
  if (!b) return '0 B'
  if (b >= 1 << 30) return (b / (1 << 30)).toFixed(1) + ' GB'
  if (b >= 1 << 20) return (b / (1 << 20)).toFixed(1) + ' MB'
  if (b >= 1 << 10) return (b / (1 << 10)).toFixed(1) + ' KB'
  return b + ' B'
}
</script>
