<template>
  <div class="min-h-screen flex items-center justify-center relative overflow-hidden bg-slate-50">

    <!-- Dynamic Geometric Square Background -->
    <div class="absolute inset-0 z-0 overflow-hidden pointer-events-none opacity-50">
      <!-- 3 floating squares -->
      <div class="absolute w-32 h-32 border-2 border-primary-400 rounded-lg animate-float-square left-[15%] bottom-[-10%]" style="animation-duration: 25s; animation-delay: 0s;"></div>
      <div class="absolute w-48 h-48 border border-accent rounded-xl animate-float-square left-[50%] bottom-[-20%]" style="animation-duration: 35s; animation-delay: 5s;"></div>
      <div class="absolute w-24 h-24 border-[3px] border-indigo-400 rounded-md animate-float-square left-[80%] bottom-[-5%]" style="animation-duration: 20s; animation-delay: 2s;"></div>
    </div>

    <!-- Soft glowing ambient blobs -->
    <div class="absolute top-1/4 left-1/4 w-[400px] h-[400px] bg-primary-400/20 rounded-full mix-blend-multiply filter blur-[80px] opacity-60 animate-blob"></div>
    <div class="absolute bottom-1/4 right-1/4 w-[500px] h-[500px] bg-accent/20 rounded-full mix-blend-multiply filter blur-[100px] opacity-60 animate-blob" style="animation-delay: 2s; animation-duration: 12s"></div>

    <!-- Form Container -->
    <!-- The width is adjusted to max-w-sm (slightly narrower) for better proportion and balance -->
    <form @submit.prevent="login" class="glass-card p-8 sm:p-10 w-[90%] max-w-sm space-y-6 relative z-10 shadow-2xl border border-white/50">

      <!-- Header -->
      <div class="text-center space-y-3 mb-8 animate-slide-up stagger-1">
        <div class="w-16 h-16 mx-auto bg-gradient-to-br from-primary-500 to-primary-700 rounded-2xl shadow-glow flex items-center justify-center transition-transform duration-500 hover:scale-110 hover:rotate-[5deg]">
          <svg class="w-8 h-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h1 class="text-3xl font-extrabold text-slate-800 tracking-tight">PigeonRelay</h1>
        <p class="text-sm font-medium text-slate-500">订阅链接管理平台</p>
      </div>

      <!-- Inputs -->
      <div class="space-y-5 animate-slide-up stagger-2">
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

      <!-- Error -->
      <div v-if="error" class="p-3 bg-danger/10 border border-danger/20 rounded-xl flex items-start gap-2 text-danger animate-fade-in stagger-3">
        <svg class="w-5 h-5 shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span class="text-sm">{{ error }}</span>
      </div>

      <!-- Submit -->
      <div class="animate-slide-up stagger-4 pt-2">
        <Button
          type="submit"
          class="w-full h-12 text-base group relative overflow-hidden font-semibold tracking-wide"
          :loading="loading"
        >
          <span class="relative z-10">登录系统</span>
        </Button>
      </div>
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