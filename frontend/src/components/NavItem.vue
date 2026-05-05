<template>
  <router-link
    :to="to"
    class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-white/70 hover:bg-white/10 hover:text-white transition-colors"
    active-class="bg-white/15 text-white"
  >
    <svg v-if="icon" class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" :d="icon" />
    </svg>
    <span>{{ label }}</span>
  </router-link>
</template>

<script setup>
defineProps({ icon: String, label: String, to: String })
</script>
