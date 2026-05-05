<template>
  <Toast />
  <div v-if="$route.path === '/login'">
    <router-view v-slot="{ Component }">
      <transition name="page" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
  </div>
  <div v-else class="flex min-h-screen">
    <SideNav />
    <main class="flex-1 p-6 lg:p-8 overflow-x-hidden overflow-y-auto relative bg-slate-50/50">
      <router-view v-slot="{ Component }">
        <transition name="page" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
  </div>
</template>

<script setup>
import SideNav from './components/SideNav.vue'
import Toast from './components/ui/Toast.vue'
</script>
