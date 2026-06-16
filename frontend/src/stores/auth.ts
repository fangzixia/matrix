import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import * as authApi from '@/api/auth'
import type { User } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(false)
  const initialized = ref(false)

  const isLoggedIn = computed(() => !!user.value)
  const isAdmin = computed(() => !!user.value?.is_admin)
  const isRoot = computed(() => !!user.value?.is_root)

  async function fetchMe() {
    loading.value = true
    try {
      user.value = await authApi.me()
    } catch {
      user.value = null
    } finally {
      loading.value = false
      initialized.value = true
    }
  }

  async function login(username: string, password: string) {
    user.value = await authApi.login(username, password)
  }

  async function logout() {
    await authApi.logout()
    user.value = null
  }

  return { user, loading, initialized, isLoggedIn, isAdmin, isRoot, fetchMe, login, logout }
})
