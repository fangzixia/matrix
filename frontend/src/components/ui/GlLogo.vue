<script setup lang="ts">
defineProps<{ showText?: boolean; size?: number }>()
</script>

<template>
  <span class="gl-logo">
    <svg
      class="gl-logo__mark"
      viewBox="0 0 36 36"
      :width="size || 24"
      :height="size || 24"
      aria-hidden="true"
    >
      <path fill="#e24329" d="M18 0C8.06 0 0 8.06 0 18s8.06 18 18 18 18-8.06 18-18S27.94 0 18 0z" />
      <path fill="#fc6d26" d="M18 4.5L6 18h4.5l7.5-9 7.5 9H30L18 4.5z" />
      <path fill="#fca326" d="M18 36c4.97 0 9.45-2.01 12.7-5.25L18 13.5 5.3 30.75A17.93 17.93 0 0 0 18 36z" />
    </svg>
    <span v-if="showText !== false" class="gl-logo__text">Matrix</span>
  </span>
</template>

<style scoped lang="scss">
.gl-logo {
  display: inline-flex;
  align-items: center;
  gap: var(--gl-spacing-2);
}

.gl-logo__text {
  font-weight: 700;
  font-size: var(--gl-font-size-lg);
  color: var(--gl-color-orange-600);
  letter-spacing: -0.01em;
}
</style>
