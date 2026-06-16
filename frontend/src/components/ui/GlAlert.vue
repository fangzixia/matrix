<script setup lang="ts">
defineProps<{
  variant?: 'info' | 'success' | 'warning' | 'danger'
}>()
</script>

<template>
  <div class="gl-alert" :class="`gl-alert--${variant || 'info'}`" role="alert">
    <slot />
  </div>
</template>

<style scoped lang="scss">
.gl-alert {
  padding: var(--gl-spacing-3) var(--gl-spacing-4);
  border-radius: var(--gl-radius);
  border: 1px solid var(--gl-border-color-default);
  background: var(--gl-background-color-default);
  font-size: var(--gl-font-size-sm);
  line-height: 1.5;
}

.gl-alert--info {
  border-color: var(--gl-color-blue-50);
  background: var(--gl-color-blue-50);
  color: var(--gl-color-blue-600);
}

.gl-alert--danger {
  border-color: var(--gl-color-red-50);
  background: var(--gl-color-red-50);
  color: var(--gl-color-red-600);
}

.gl-alert--success {
  border-color: var(--gl-color-green-50);
  background: var(--gl-color-green-50);
  color: var(--gl-color-green-600);
}

.gl-alert--warning {
  border-color: #ffec99;
  background: #fffbeb;
  color: #7d4e00;
}
</style>
