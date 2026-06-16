<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from '@/layouts/AppShell.vue'
import AuthLayout from '@/layouts/AuthLayout.vue'

const route = useRoute()
const layout = computed(() => {
  if (route.meta.layout === 'auth') return 'auth'
  if (route.matched.some((r) => r.meta.admin)) return 'bare'
  return 'app'
})
</script>

<template>
  <AuthLayout v-if="layout === 'auth'">
    <router-view />
  </AuthLayout>
  <AppShell v-else-if="layout === 'app'">
    <router-view />
  </AppShell>
  <router-view v-else />
</template>
