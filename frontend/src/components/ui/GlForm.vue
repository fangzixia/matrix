<script setup lang="ts">
defineProps<{
  label?: string
  error?: string
}>()
</script>

<template>
  <label class="gl-form-field">
    <span v-if="label" class="gl-form-field__label gl-label">{{ label }}</span>
    <slot />
    <span v-if="error" class="gl-form-field__error">{{ error }}</span>
  </label>
</template>

<style scoped lang="scss">
.gl-form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.gl-form-field__label {
  margin-bottom: 0;
}

.gl-form-field__error {
  color: #ae1800;
  font-size: 12px;
}
</style>
