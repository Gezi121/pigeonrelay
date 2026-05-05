<template>
  <div class="glass-card" :class="[padding ? 'p-6' : '', className]">
    <slot></slot>
  </div>
</template>

<script setup>
defineProps({
  padding: { type: Boolean, default: true },
  className: { type: String, default: '' }
})
</script>
