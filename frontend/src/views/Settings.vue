<template>
  <div class="max-w-4xl mx-auto animate-fade-in">
    <div class="mb-8">
      <h1 class="text-2xl font-bold text-slate-800 tracking-tight">系统设置</h1>
      <p class="text-sm text-slate-500 mt-1">管理 Clash 订阅模板、管理员账号。当前版本 v{{ version || '...' }}</p>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading" class="space-y-6">
      <Card v-for="i in 2" :key="i" class="animate-pulse"><div class="h-5 bg-slate-200 rounded w-1/3 mb-3"></div><div class="h-4 bg-slate-100 rounded w-2/3"></div></Card>
    </div>

    <div v-else class="space-y-6">
      <!-- Clash 模板 -->
      <Card padding>
        <h2 class="text-lg font-bold text-slate-800 mb-2">Clash 订阅模板</h2>
        <p class="text-sm text-slate-500 mb-4">节点列表将替换模板中的 <code class="bg-slate-100 px-1 rounded text-xs font-mono">${proxies}</code> 占位符。</p>
        <textarea v-model="settingsForm.clash_template" rows="12" class="input-glass font-mono text-xs" spellcheck="false"></textarea>
        <div class="mt-3 flex gap-2">
          <Button variant="secondary" @click="resetTemplate">恢复默认</Button>
          <Button @click="saveSettings" :loading="saving">保存模板</Button>
        </div>
        <div v-if="settingsSaved" class="text-xs text-success mt-2">已保存</div>
      </Card>

      <!-- 测速 Token -->
      <Card padding>
        <h2 class="text-lg font-bold text-slate-800 mb-2">测速上报 Token</h2>
        <p class="text-sm text-slate-500 mb-4">测速客户端使用此 Token 鉴权上报延迟数据。修改后需同步更新客户端 LATENCY_TOKEN。</p>
        <div class="flex gap-2">
          <Input v-model="settingsForm.latency_token" placeholder="default-latency-token" />
          <Button @click="saveSettings" :loading="saving">保存</Button>
        </div>
        <div v-if="settingsSaved" class="text-xs text-success mt-2">已保存</div>
        <p class="text-xs text-slate-400 mt-2">若不设置，默认使用环境变量 <code class="bg-slate-100 px-1 rounded">LATENCY_TOKEN</code>，回退到 <code class="bg-slate-100 px-1 rounded">default-latency-token</code>。</p>
      </Card>

      <!-- CF 优选 -->
      <Card padding>
        <h2 class="text-lg font-bold text-slate-800 mb-2">CF 优选配置</h2>
        <p class="text-sm text-slate-500 mb-4">每 30 分钟将本地优选子域名的 A 记录更新为最优 CF IP（灰云）。Token 在 Cloudflare → My Profile → API Tokens → Edit zone DNS。</p>

        <!-- Token row -->
        <div class="flex gap-2 mb-3">
          <div class="relative flex-1">
            <Input v-model="settingsForm.cf_api_token" :type="showToken ? 'text' : 'password'" :placeholder="cfTokenSet ? 'Token 已保存，输入新值覆盖' : '粘贴 CF API Token'" class="w-full pr-10" />
            <button type="button" class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600" @click="showToken = !showToken">
              <svg class="w-4 h-4" :class="{ hidden: showToken }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/></svg>
              <svg class="w-4 h-4" :class="{ hidden: !showToken }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/></svg>
            </button>
          </div>
          <Badge v-if="cfTokenSet" variant="success" class="self-center shrink-0">已设置</Badge>
          <Button variant="secondary" size="sm" @click="testCFToken" :loading="testingToken">测试</Button>
          <Button @click="saveSettings" :loading="saving">保存</Button>
        </div>
        <p v-if="tokenResult" class="text-xs mb-2" :class="tokenResult.valid ? 'text-emerald-600' : 'text-red-500'">{{ tokenResult.message }}</p>

        <!-- Zone ID row -->
        <div class="flex gap-2 mb-3 items-center">
          <label class="text-sm text-slate-600 w-20 shrink-0">Zone ID</label>
          <Input v-model="settingsForm.cf_zone_id" placeholder="自动发现，也可手动填入" class="flex-1" />
          <span class="text-xs text-slate-400 shrink-0">可选</span>
        </div>

        <!-- CF interval row -->
        <div class="flex gap-2 mb-3 items-center">
          <label class="text-sm text-slate-600 w-20 shrink-0">更新间隔</label>
          <Input v-model="settingsForm.cf_update_minutes" placeholder="30" class="w-24" />
          <span class="text-xs text-slate-400">分钟（最小 5）</span>
        </div>

        <div v-if="settingsSaved" class="text-xs text-success mb-3">已保存</div>

        <!-- Auto domain list -->
        <div v-if="cfDomains.length > 0" class="bg-slate-50 rounded-lg p-3 border border-slate-100 mb-2">
          <p class="text-xs font-medium text-slate-600 mb-2">优选子域名（从本地优选自动读取）</p>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="d in cfDomains" :key="d.full_domain" variant="info">{{ d.full_domain }}</Badge>
          </div>
        </div>
        <div v-else class="text-xs text-slate-400">暂无本地优选子域名，先在"本地优选看板"添加。</div>
      </Card>

      <!-- 管理员账号 -->
      <Card padding>
        <h2 class="text-lg font-bold text-slate-800 mb-2">管理员账号</h2>
        <p class="text-sm text-slate-500 mb-4">修改当前管理员账号的用户名和密码。密码留空保持不变。</p>
        <div class="grid grid-cols-2 gap-4">
          <div><label class="block text-sm font-medium text-slate-700 mb-1">用户名</label><Input v-model="adminForm.username" placeholder="admin" /></div>
          <div><label class="block text-sm font-medium text-slate-700 mb-1">新密码（留空保持原密码）</label><Input v-model="adminForm.password" type="password" placeholder="留空则不改" /></div>
        </div>
        <div class="mt-3">
          <Button @click="saveAdmin" :loading="savingAdmin">更新账号</Button>
        </div>
        <div v-if="adminSaved" class="text-xs text-success mt-2">账号已更新</div>
      </Card>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api.js'
import Card from '../components/ui/Card.vue'
import Button from '../components/ui/Button.vue'
import Input from '../components/ui/Input.vue'

const DEFAULT_TEMPLATE = `port: 7890
socks-port: 7891
allow-lan: true
mode: rule
log-level: info
external-controller: 127.0.0.1:9090

# 节点配置
proxies:
\${proxies}

# 策略组配置
proxy-groups:
  - name: 🚀 节点选择
    type: select
    proxies:
      - ♻️ 自动选择
\${proxy-names}
    url: http://www.gstatic.com/generate_204
    interval: 300

  - name: ♻️ 自动选择
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    proxies:
\${proxy-names}

# 路由规则
rules:
  - GEOIP,LAN,DIRECT
  - GEOIP,CN,DIRECT
  - MATCH,🚀 节点选择`

const version = ref('')
const settingsForm = ref({ clash_template: '', base_domain: '', latency_token: '', cf_api_token: '', cf_zone_id: '', cf_update_minutes: '30' })
const adminForm = ref({ username: '', password: '' })
const loading = ref(true)
const saving = ref(false)
const savingAdmin = ref(false)
const testingToken = ref(false)
const tokenResult = ref(null)
const settingsSaved = ref(false)
const adminSaved = ref(false)
const showToken = ref(false)
const cfTokenSet = ref(false)
const cfDomains = ref([])

onMounted(load)

async function load() {
  try {
    const s = await api.request('GET', '/settings')
    version.value = s.version || '1.0.0'
    settingsForm.value = { clash_template: s.clash_template || DEFAULT_TEMPLATE, base_domain: s.base_domain || '', latency_token: s.latency_token || '', cf_api_token: '', cf_zone_id: s.cf_zone_id || '', cf_update_minutes: s.cf_update_minutes || '30' }
  cfTokenSet.value = !!s.cf_token_set
  cfDomains.value = s.cf_domains || []
  } catch { settingsForm.value.clash_template = DEFAULT_TEMPLATE }
  finally { loading.value = false }
}

async function saveSettings() {
  saving.value = true; settingsSaved.value = false
  try {
    await api.request('PUT', '/settings', settingsForm.value)
    settingsSaved.value = true; setTimeout(() => settingsSaved.value = false, 3000)
  } catch (e) { window.$toast?.error('保存失败', e.message) }
  finally { saving.value = false }
}

async function testCFToken() {
  const token = settingsForm.value.cf_api_token
  if (!token) { window.$toast?.warning('请先填写 Token'); return }
  testingToken.value = true
  try {
    tokenResult.value = await api.request('POST', '/cf/verify-token', { token })
  } catch (e) {
    tokenResult.value = { valid: false, message: e.message }
  } finally {
    testingToken.value = false
  }
}

function resetTemplate() {
  settingsForm.value.clash_template = DEFAULT_TEMPLATE
}

async function saveAdmin() {
  if (!adminForm.value.username) { window.$toast?.warning('用户名必填'); return }
  savingAdmin.value = true; adminSaved.value = false
  try {
    await api.request('PUT', '/admin/account', adminForm.value)
    adminSaved.value = true
    adminForm.value.password = ''
    setTimeout(() => adminSaved.value = false, 3000)
    window.$toast?.success('已更新', '管理员账号已保存，请使用新凭据重新登录')
  } catch (e) { window.$toast?.error('更新失败', e.message) }
  finally { savingAdmin.value = false }
}
</script>
