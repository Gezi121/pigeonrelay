<template>
  <span
    class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
    :class="variants[variant] || variants.default"
  >
    <span v-if="dot" class="w-1.5 h-1.5 rounded-full mr-1.5" :class="dotColors[variant] || dotColors.default"></span>
    <slot></slot>
  </span>
</template>

<script setup>
defineProps({
  variant: { type: String, default: 'default' }, // success, warning, danger, primary, default
  dot: { type: Boolean, default: false }
})

const variants = {
  success: 'bg-green-50 text-green-700 border border-green-200',
  warning: 'bg-yellow-50 text-yellow-700 border border-yellow-200',
  danger: 'bg-red-50 text-red-700 border border-red-200',
  primary: 'bg-primary-50 text-primary-700 border border-primary-200',
  default: 'bg-slate-100 text-slate-600 border border-slate-200'
}

const dotColors = {
  success: 'bg-green-500',
  warning: 'bg-yellow-500',
  danger: 'bg-red-500',
  primary: 'bg-primary-500',
  default: 'bg-slate-400'
}
</script>
