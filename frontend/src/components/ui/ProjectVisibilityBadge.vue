<script setup lang="ts">
import type { ProjectVisibility } from '@/api/projects'

defineProps<{ visibility: ProjectVisibility }>()

const labels: Record<ProjectVisibility, string> = {
  private: '私有',
  internal: '内部',
  public: '公开',
}

const titles: Record<ProjectVisibility, string> = {
  private: '私有 — 仅项目成员可访问',
  internal: '内部 — 所有已登录用户可访问',
  public: '公开 — 所有已登录用户可访问',
}
</script>

<template>
  <span class="visibility-badge" :title="titles[visibility]">
    <svg v-if="visibility === 'private'" width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
      <path fill="currentColor" d="M8 1a3 3 0 0 0-3 3v2H4a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V7a1 1 0 0 0-1-1h-1V4a3 3 0 0 0-3-3zm-1 5V4a1 1 0 1 1 2 0v2H7z"/>
    </svg>
    <svg v-else width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
      <path fill="currentColor" d="M8 1C4.5 1 1.73 3.61 1 7c.73 3.39 3.5 6 7 6s6.27-2.61 7-6c-.73-3.39-3.5-6-7-6zm0 10a4 4 0 1 1 0-8 4 4 0 0 1 0 8zm0-2a2 2 0 1 0 0-4 2 2 0 0 0 0 4z"/>
    </svg>
    {{ labels[visibility] }}
  </span>
</template>

<style scoped lang="scss">
.visibility-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--gl-text-color-subtle);
  font-weight: 500;
}
</style>
