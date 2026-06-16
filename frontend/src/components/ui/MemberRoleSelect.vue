<script setup lang="ts">
defineProps<{ modelValue: import('@/api/projects').MemberRole; showHints?: boolean }>()
defineEmits<{ 'update:modelValue': [value: import('@/api/projects').MemberRole] }>()

const roles: { value: import('@/api/projects').MemberRole; label: string; hint: string }[] = [
  { value: 'guest', label: '访客', hint: '只读访问项目、仓库与 Runs' },
  { value: 'reporter', label: '报告者', hint: '访客 + Git 拉取' },
  { value: 'developer', label: '开发者', hint: '创建 Run、对话写入、Git 拉取' },
  { value: 'maintainer', label: '维护者', hint: '管理设置与成员、Git 推送' },
  { value: 'owner', label: '所有者', hint: '完全控制，含删除项目' },
]
</script>

<template>
  <select
    class="gl-select"
    :value="modelValue"
    :title="roles.find((r) => r.value === modelValue)?.hint"
    @change="$emit('update:modelValue', ($event.target as HTMLSelectElement).value as import('@/api/projects').MemberRole)"
  >
    <option v-for="r in roles" :key="r.value" :value="r.value" :title="r.hint">
      {{ r.label }}{{ showHints ? ` — ${r.hint}` : '' }}
    </option>
  </select>
</template>

<style scoped lang="scss">
.gl-select {
  padding: 8px 12px;
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  background: var(--gl-background-color-default);
  max-width: 100%;
  font: inherit;
  font-size: var(--gl-font-size-sm);

  &:focus {
    outline: none;
    border-color: var(--gl-color-blue-500);
    box-shadow: var(--gl-focus-ring);
  }
}
</style>
