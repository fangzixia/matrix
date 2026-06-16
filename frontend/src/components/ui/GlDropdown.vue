<script setup lang="ts">
defineProps<{ label: string; variant?: 'default' | 'nav' }>()
const open = defineModel<boolean>('open', { default: false })
</script>

<template>
  <div class="gl-dropdown" :class="{ 'gl-dropdown--nav': variant === 'nav' }">
    <button type="button" class="gl-dropdown__toggle" @click="open = !open">
      {{ label }}
      <svg class="gl-dropdown__chevron" width="12" height="12" viewBox="0 0 16 16" aria-hidden="true">
        <path fill="currentColor" d="M4 6l4 4 4-4H4z" />
      </svg>
    </button>
    <div v-if="open" class="gl-dropdown__menu">
      <slot />
    </div>
  </div>
</template>

<style scoped lang="scss">
.gl-dropdown {
  position: relative;
  display: inline-block;
}

.gl-dropdown__toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  border: 1px solid transparent;
  border-radius: var(--gl-radius);
  background: transparent;
  font: inherit;
  font-size: var(--gl-font-size-sm);
  font-weight: 600;
  color: var(--gl-text-color-default);
  cursor: pointer;
  transition: background 0.12s;

  &:hover {
    background: var(--gl-background-color-subtle);
  }
}

.gl-dropdown--nav .gl-dropdown__toggle {
  font-size: var(--gl-font-size-md);
}

.gl-dropdown__chevron {
  opacity: 0.6;
}

.gl-dropdown__menu {
  position: absolute;
  left: 0;
  top: calc(100% + 4px);
  min-width: 200px;
  background: var(--gl-background-color-default);
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  box-shadow: var(--gl-shadow-dropdown);
  z-index: 300;
  overflow: hidden;
}
</style>
