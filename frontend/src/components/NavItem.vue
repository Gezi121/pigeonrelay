<template>
  <router-link
    :to="to"
    class="relative flex items-center gap-3 px-3 py-2.5 rounded-xl text-white/70 hover:text-white transition-colors group overflow-hidden"
    active-class="text-white font-medium"
  >
    <!-- Background active state indicator -->
    <div class="absolute inset-0 bg-white/15 opacity-0 group-[.router-link-active]:opacity-100 transition-opacity duration-300 z-0"></div>

    <!-- Hover sliding highlight -->
    <div class="absolute inset-0 bg-gradient-to-r from-white/10 to-transparent -translate-x-full group-hover:translate-x-0 transition-transform duration-300 ease-out z-0"></div>

    <svg v-if="icon" class="w-5 h-5 shrink-0 relative z-10 transition-transform duration-300 group-hover:scale-110" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" :d="icon" />
    </svg>
    <span class="relative z-10">{{ label }}</span>
  </router-link>
</template>

<script setup>
defineProps({ icon: String, label: String, to: String })
</script>
