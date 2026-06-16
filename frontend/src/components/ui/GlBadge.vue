<script setup lang="ts">
defineProps<{ variant?: 'neutral' | 'success' | 'warning' | 'danger' | 'info' }>()
</script>

<template>
  <span class="gl-badge" :class="`gl-badge--${variant || 'neutral'}`">
    <slot />
  </span>
</template>

<style scoped lang="scss">
.gl-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: var(--gl-font-size-sm);
  font-weight: 600;
  line-height: 1.4;
  white-space: nowrap;
}

.gl-badge--neutral {
  background: var(--gl-background-color-strong);
  color: var(--gl-text-color-default);
}

.gl-badge--info {
  background: var(--gl-color-blue-50);
  color: var(--gl-color-blue-600);
}

.gl-badge--success {
  background: var(--gl-color-green-50);
  color: var(--gl-color-green-600);
}

.gl-badge--warning {
  background: #ffec99;
  color: #7d4e00;
}

.gl-badge--danger {
  background: var(--gl-color-red-50);
  color: var(--gl-color-red-600);
}
</style>
