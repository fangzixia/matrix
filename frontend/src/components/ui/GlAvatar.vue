<script setup lang="ts">
defineProps<{
  name?: string
  src?: string
  size?: number
  square?: boolean
}>()
</script>

<template>
  <span
    class="gl-avatar"
    :class="{ 'gl-avatar--square': square }"
    :style="{ width: (size || 32) + 'px', height: (size || 32) + 'px', fontSize: Math.max(10, (size || 32) * 0.38) + 'px' }"
  >
    <img v-if="src" :src="src" :alt="name || 'avatar'" />
    <span v-else class="gl-avatar__initial">{{ (name || '?').slice(0, 1).toUpperCase() }}</span>
  </span>
</template>

<style scoped lang="scss">
.gl-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--gl-color-orange-500), var(--gl-color-orange-600));
  color: #fff;
  font-weight: 700;
  overflow: hidden;
  flex-shrink: 0;
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.06);
}

.gl-avatar--square {
  border-radius: var(--gl-radius);
}

.gl-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.gl-avatar__initial {
  line-height: 1;
}
</style>
