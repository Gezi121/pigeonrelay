<template>
  <button
    :type="type"
    :disabled="disabled || loading"
    :class="[
      baseClass,
      variantClass[variant],
      sizeClass[size],
      className
    ]"
    @click="$emit('click', $event)"
  >
    <svg v-if="loading" class="animate-spin -ml-1 mr-2 h-4 w-4 text-current" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>
    <svg v-if="icon && !loading" class="w-4 h-4 mr-2 -ml-1 relative z-10" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="icon" />
    </svg>
    <div v-if="variant === 'primary'" class="absolute inset-0 bg-white/20 translate-y-full group-hover:translate-y-0 transition-transform duration-300 ease-out z-0"></div>
    <span class="relative z-10"><slot></slot></span>
  </button>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  type: { type: String, default: 'button' },
  variant: { type: String, default: 'primary' }, // primary, secondary, danger, ghost
  size: { type: String, default: 'md' }, // sm, md, lg
  loading: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  icon: { type: String, default: '' },
  className: { type: String, default: '' }
})

defineEmits(['click'])

const baseClass = 'inline-flex items-center justify-center transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none disabled:shadow-none'

const variantClass = {
  primary: 'bg-gradient-to-r from-primary-600 to-primary-500 text-white rounded-xl shadow-md shadow-primary-500/20 hover:shadow-glow hover:shadow-primary-500/40 hover:-translate-y-0.5 active:translate-y-0 active:scale-95 group relative overflow-hidden',
  secondary: 'bg-white/80 text-primary-700 border border-primary-200 rounded-xl shadow-sm hover:bg-primary-50 hover:border-primary-300 hover:text-primary-800 hover:-translate-y-0.5 active:translate-y-0 active:scale-95 group relative overflow-hidden',
  danger: 'bg-white/80 text-danger border border-red-200 rounded-xl shadow-sm hover:bg-red-50 hover:border-red-300 hover:text-red-600 hover:-translate-y-0.5 active:translate-y-0 active:scale-95 group relative overflow-hidden',
  ghost: 'text-slate-600 hover:text-primary hover:bg-primary-50 rounded-xl active:scale-95'
}

const sizeClass = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-5 py-2.5 text-sm font-medium',
  lg: 'px-6 py-3 text-base font-medium'
}
</script>
