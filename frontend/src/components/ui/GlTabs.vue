<script setup lang="ts">
defineProps<{ tabs: { key: string; label: string; to: string }[] }>()
</script>

<template>
  <nav class="gl-tabs">
    <router-link
      v-for="tab in tabs"
      :key="tab.key"
      :to="tab.to"
      class="gl-tabs__item"
      active-class="gl-tabs__item--active"
    >
      {{ tab.label }}
    </router-link>
  </nav>
</template>

<style scoped lang="scss">
.gl-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--gl-border-color-default);
  margin-bottom: 16px;
}

.gl-tabs__item {
  padding: 10px 14px;
  color: var(--gl-text-color-subtle);
  border-bottom: 2px solid transparent;
  text-decoration: none;
}

.gl-tabs__item:hover {
  color: var(--gl-text-color-default);
  text-decoration: none;
}

.gl-tabs__item--active {
  color: var(--gl-text-color-default);
  border-bottom-color: var(--gl-color-orange-500);
  font-weight: 600;
}
</style>
