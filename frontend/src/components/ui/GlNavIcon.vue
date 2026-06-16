<script setup lang="ts">
defineProps<{ name: 'overview' | 'chat' | 'runs' | 'repository' | 'settings' | 'users' | 'dashboard' }>()
</script>

<template>
  <svg class="gl-nav-icon" width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
    <template v-if="name === 'overview'">
      <path fill="currentColor" d="M2 2h5v5H2V2zm7 0h5v5H9V2zM2 9h5v5H2V9zm7 0h5v5H9V9z" opacity="0.9" />
    </template>
    <template v-else-if="name === 'chat'">
      <path fill="currentColor" d="M2 2h12v8H6l-3 3V2z" />
    </template>
    <template v-else-if="name === 'runs'">
      <path fill="currentColor" d="M3 2h10l-1 10H4L3 2zm2 2v6h6V4H5zm1 1h4v4H6V5z" />
    </template>
    <template v-else-if="name === 'repository'">
      <path fill="currentColor" d="M2 3h12v2H2V3zm0 4h12v2H2V7zm0 4h8v2H2v-2z" />
    </template>
    <template v-else-if="name === 'settings'">
      <path fill="currentColor" d="M8 1a3 3 0 1 0 0 6 3 3 0 0 0 0-6zm-6 9a1 1 0 0 1 1-1h10a1 1 0 1 1 0 2H3a1 1 0 0 1-1-1z" />
    </template>
    <template v-else-if="name === 'users'">
      <path fill="currentColor" d="M8 2a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5zM3 12c0-2.2 2.2-4 5-4s5 1.8 5 4v1H3v-1z" />
    </template>
    <template v-else>
      <path fill="currentColor" d="M2 2h5v5H2V2zm7 0h5v5H9V2zM2 9h5v5H2V9zm7 0h5v5H9V9z" />
    </template>
  </svg>
</template>

<style scoped lang="scss">
.gl-nav-icon {
  flex-shrink: 0;
  opacity: 0.72;
}
</style>
