<script setup lang="ts">
defineProps<{
  page: number
  totalPages: number
}>()
defineEmits<{ change: [page: number] }>()
</script>

<template>
  <nav class="gl-pagination">
    <button type="button" :disabled="page <= 1" @click="$emit('change', page - 1)">上一页</button>
    <span>{{ page }} / {{ totalPages }}</span>
    <button type="button" :disabled="page >= totalPages" @click="$emit('change', page + 1)">下一页</button>
  </nav>
</template>

<style scoped lang="scss">
.gl-pagination {
  display: flex;
  align-items: center;
  gap: 12px;
}

button {
  padding: 6px 12px;
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  background: var(--gl-background-color-default);
  cursor: pointer;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
