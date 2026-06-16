<script setup lang="ts">
defineProps<{ open: boolean; title: string }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <div v-if="open" class="gl-modal-backdrop" @click.self="emit('close')">
    <div class="gl-modal">
      <header class="gl-modal__header">
        <h3>{{ title }}</h3>
        <button type="button" class="gl-modal__close" @click="emit('close')">×</button>
      </header>
      <div class="gl-modal__body">
        <slot />
      </div>
      <footer v-if="$slots.footer" class="gl-modal__footer">
        <slot name="footer" />
      </footer>
    </div>
  </div>
</template>

<style scoped lang="scss">
.gl-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.gl-modal {
  width: min(520px, 92vw);
  background: var(--gl-background-color-default);
  border-radius: var(--gl-radius);
  box-shadow: var(--gl-shadow);
}

.gl-modal__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid var(--gl-border-color-default);
}

.gl-modal__header h3 { margin: 0; }

.gl-modal__close {
  border: none;
  background: transparent;
  font-size: 24px;
  cursor: pointer;
}

.gl-modal__body { padding: 16px; }

.gl-modal__footer {
  padding: 16px;
  border-top: 1px solid var(--gl-border-color-default);
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
