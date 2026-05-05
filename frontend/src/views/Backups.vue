<template>
  <div class="max-w-6xl mx-auto animate-fade-in">
    <div class="flex items-center justify-between mb-8">
      <div>
        <h1 class="text-2xl font-bold text-slate-800 tracking-tight">配置备份</h1>
        <p class="text-sm text-slate-500 mt-1">每次重新生成配置时系统会自动备份，您也可以手动备份当前状态。</p>
      </div>
      <Button icon="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" @click="createBackup">立即备份</Button>
    </div>

    <!-- 加载骨架屏 -->
    <div v-if="loading" class="space-y-4">
      <Card v-for="i in 3" :key="i" class="h-20 animate-pulse bg-slate-100" />
    </div>

    <Card v-else-if="backups.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
      </div>
      <p class="text-slate-600 font-medium mb-1">暂无备份记录</p>
      <p class="text-sm text-slate-400 mb-6">生成配置或点击上方按钮进行手动备份</p>
      <Button variant="secondary" @click="createBackup">立即创建备份</Button>
    </Card>

    <TransitionGroup v-else name="list" tag="div" class="space-y-3">
      <Card v-for="b in backups" :key="b.id" class="group flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-4 hover:shadow-md transition-shadow">
        <div class="flex items-start gap-4">
          <div class="w-10 h-10 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center shrink-0">
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
            </svg>
          </div>
          <div>
            <div class="flex items-center gap-2 mb-1">
              <p class="font-bold text-slate-800">备份 #{{ b.id }}</p>
              <Badge :variant="b.backup_type === 'manual' ? 'primary' : 'default'" dot>{{ b.backup_type }}</Badge>
            </div>
            <p class="text-xs text-slate-400 font-mono">{{ new Date(b.created_at).toLocaleString() }}</p>
          </div>
        </div>
        <Button variant="secondary" size="sm" icon="M15 12a3 3 0 11-6 0 3 3 0 016 0z" @click="restore(b.id)">
          查看内容
        </Button>
      </Card>
    </TransitionGroup>

    <!-- 恢复详情弹窗 -->
    <Modal v-model="showRestore" title="备份内容详情">
      <div class="space-y-4">
        <p class="text-sm text-slate-500">
          您可以直接复制这些内容用于恢复，或另存为文件。
        </p>
        <div class="relative group">
          <pre class="text-xs text-slate-600 max-h-96 overflow-auto bg-slate-50 border border-slate-100 p-4 rounded-xl font-mono whitespace-pre-wrap break-all">{{ restored }}</pre>
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
        <Button variant="primary" @click="showRestore = false">关闭</Button>
      </template>
    </Modal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'
import Card from '../components/ui/Card.vue'
import Button from '../components/ui/Button.vue'
import Badge from '../components/ui/Badge.vue'
import Modal from '../components/ui/Modal.vue'

const backups = ref([])
const loading = ref(true)
const restored = ref(null)
const showRestore = ref(false)

onMounted(load)

async function load() {
  loading.value = true
  try {
    backups.value = await api.listBackups()
  } catch (e) {
    window.$toast?.error('加载失败', e.message)
  } finally {
    loading.value = false
  }
}

async function createBackup() {
  try {
    await api.createBackup({ backup_type: 'manual', config_content: '手动备份占位' })
    window.$toast?.success('备份成功', '已创建新的手动备份')
    await load()
  } catch (e) {
    window.$toast?.error('备份失败', e.message)
  }
}

async function restore(id) {
  try {
    const res = await api.restoreBackup(id)
    restored.value = res.config_content
    showRestore.value = true
  } catch (e) {
    window.$toast?.error('读取失败', e.message)
  }
}

async function copyConfig() {
  try {
    await navigator.clipboard.writeText(restored.value)
    window.$toast?.success('已复制', '内容已复制到剪贴板')
  } catch {
    const el = document.createElement('textarea')
    el.value = restored.value
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    window.$toast?.success('已复制', '内容已复制到剪贴板')
  }
}
</script>
