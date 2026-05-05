import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: null,
  }),
  actions: {
    login(token) {
      this.token = token
    },
    logout() {
      this.token = null
      this.user = null
    },
  },
})
