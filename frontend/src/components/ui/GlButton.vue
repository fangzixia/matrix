<script setup lang="ts">
defineProps<{
  variant?: 'primary' | 'secondary' | 'danger' | 'link'
  disabled?: boolean
  type?: 'button' | 'submit'
}>()
</script>

<template>
  <button
    class="gl-btn"
    :class="[`gl-btn--${variant || 'secondary'}`, { 'gl-btn--disabled': disabled }]"
    :disabled="disabled"
    :type="type || 'button'"
  >
    <slot />
  </button>
</template>

<style scoped lang="scss">
.gl-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: var(--gl-radius);
  border: 1px solid var(--gl-border-color-default);
  background: var(--gl-background-color-default);
  color: var(--gl-text-color-default);
  font: inherit;
  font-size: var(--gl-font-size-sm);
  font-weight: 600;
  line-height: 1.2;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, box-shadow 0.12s;
  white-space: nowrap;

  &:hover:not(:disabled) {
    background: var(--gl-background-color-subtle);
    border-color: var(--gl-border-color-strong);
  }

  &:focus-visible {
    outline: none;
    box-shadow: var(--gl-focus-ring);
  }
}

.gl-btn--primary {
  background: var(--gl-color-orange-500);
  border-color: var(--gl-color-orange-600);
  color: #fff;

  &:hover:not(:disabled) {
    background: var(--gl-color-orange-600);
    border-color: var(--gl-color-orange-700);
  }
}

.gl-btn--danger {
  color: var(--gl-color-red-600);
  border-color: var(--gl-color-red-50);
  background: var(--gl-color-red-50);

  &:hover:not(:disabled) {
    background: #fdd;
    border-color: var(--gl-color-red-500);
  }
}

.gl-btn--link {
  border: none;
  background: transparent;
  color: var(--gl-text-color-link);
  padding: 4px 8px;
  font-weight: 600;

  &:hover:not(:disabled) {
    background: transparent;
    text-decoration: underline;
  }
}

.gl-btn--disabled,
.gl-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
