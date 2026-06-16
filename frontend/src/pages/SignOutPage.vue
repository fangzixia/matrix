<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const loading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    await auth.logout()
  } finally {
    loading.value = false
    router.replace({ name: 'sign-in' })
  }
})
</script>

<template>
  <p class="muted">{{ loading ? '退出中…' : '正在跳转…' }}</p>
</template>
