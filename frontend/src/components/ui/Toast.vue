<template>
  <Teleport to="body">
    <TransitionGroup
      name="list"
      tag="div"
      class="fixed top-4 right-4 z-50 flex flex-col gap-3 w-full max-w-sm pointer-events-none"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="glass-card px-4 py-3 flex items-start shadow-xl border pointer-events-auto"
        :class="bgColors[toast.type]"
      >
        <div class="shrink-0 pt-0.5" :class="iconColors[toast.type]">
          <!-- Success -->
          <svg v-if="toast.type === 'success'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <!-- Error -->
          <svg v-else-if="toast.type === 'error'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <!-- Warning -->
          <svg v-else-if="toast.type === 'warning'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <!-- Info -->
          <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div class="ml-3 w-0 flex-1">
          <p class="text-sm font-medium text-slate-800">{{ toast.title }}</p>
          <p v-if="toast.message" class="mt-1 text-xs text-slate-500 line-clamp-3">{{ toast.message }}</p>
        </div>
        <button @click="remove(toast.id)" class="ml-4 shrink-0 text-slate-400 hover:text-slate-600 transition-colors">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </TransitionGroup>
  </Teleport>
</template>

<script setup>
import { ref } from 'vue'

const toasts = ref([])
let seed = 0

const bgColors = {
  success: 'bg-green-50/90 border-green-200/50',
  error: 'bg-red-50/90 border-red-200/50',
  warning: 'bg-yellow-50/90 border-yellow-200/50',
  info: 'bg-white/90 border-white/50'
}

const iconColors = {
  success: 'text-green-500',
  error: 'text-red-500',
  warning: 'text-yellow-500',
  info: 'text-primary-500'
}

function show({ title, message, type = 'info', duration = 4000 }) {
  const id = seed++
  toasts.value.push({ id, title, message, type })
  if (duration > 0) {
    setTimeout(() => remove(id), duration)
  }
}

function remove(id) {
  const index = toasts.value.findIndex(t => t.id === id)
  if (index > -1) {
    toasts.value.splice(index, 1)
  }
}

// Expose globally
if (typeof window !== 'undefined') {
  window.$toast = {
    success: (title, message, duration) => show({ title, message, type: 'success', duration }),
    error: (title, message, duration) => show({ title, message, type: 'error', duration }),
    warning: (title, message, duration) => show({ title, message, type: 'warning', duration }),
    info: (title, message, duration) => show({ title, message, type: 'info', duration }),
  }
}
</script>
