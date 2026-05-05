<template>
  <div class="min-h-screen flex items-center justify-center relative overflow-hidden">
    <!-- Animated background elements -->
    <div class="absolute top-1/4 left-1/4 w-96 h-96 bg-primary-400/20 rounded-full mix-blend-multiply filter blur-3xl opacity-70 animate-float"></div>
    <div class="absolute bottom-1/4 right-1/4 w-96 h-96 bg-accent/20 rounded-full mix-blend-multiply filter blur-3xl opacity-70 animate-float" style="animation-delay: -3s"></div>

    <form @submit.prevent="login" class="glass-card p-10 w-full max-w-md space-y-6 relative z-10 animate-slide-up">
      <div class="text-center space-y-2 mb-8">
        <div class="w-16 h-16 mx-auto bg-gradient-to-br from-primary-500 to-primary-700 rounded-2xl shadow-glow flex items-center justify-center mb-6">
          <svg class="w-8 h-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h1 class="text-3xl font-bold text-slate-800 tracking-tight">PigeonRelay</h1>
        <p class="text-sm text-slate-500">订阅链接管理与二次加工平台</p>
      </div>

      <div class="space-y-4">
        <Input
          v-model="username"
          placeholder="用户名"
          icon="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
          :disabled="loading"
        />
        <Input
          v-model="password"
          type="password"
          placeholder="密码"
          icon="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
          :disabled="loading"
        />
      </div>

      <div v-if="error" class="p-3 bg-danger/10 border border-danger/20 rounded-xl flex items-start gap-2 text-danger animate-fade-in">
        <svg class="w-5 h-5 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span class="text-sm">{{ error }}</span>
      </div>

      <Button
        type="submit"
        class="w-full h-12 text-base"
        :loading="loading"
      >
        登录系统
      </Button>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import Input from '../components/ui/Input.vue'
import Button from '../components/ui/Button.vue'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function login() {
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }

  error.value = ''
  loading.value = true

  try {
    await auth.login(username.value, password.value)
    router.push('/')
  } catch (e) {
    error.value = '用户名或密码错误，请重试'
  } finally {
    loading.value = false
  }
}
</script>
