<script setup lang="ts">
export interface Column {
  key: string
  label: string
}

defineProps<{
  columns: Column[]
  rows: Record<string, unknown>[]
  emptyText?: string
}>()
</script>

<template>
  <div class="gl-table-wrap">
    <table class="gl-table">
      <thead>
        <tr>
          <th v-for="col in columns" :key="col.key">{{ col.label }}</th>
          <th v-if="$slots.actions" class="gl-table__actions-col">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!rows.length">
          <td :colspan="columns.length + ($slots.actions ? 1 : 0)" class="gl-table__empty">
            {{ emptyText || '暂无数据' }}
          </td>
        </tr>
        <tr v-for="(row, i) in rows" :key="i" class="gl-table__row">
          <td v-for="col in columns" :key="col.key">
            <slot :name="`cell-${col.key}`" :row="row">{{ row[col.key] }}</slot>
          </td>
          <td v-if="$slots.actions" class="gl-table__actions">
            <slot name="actions" :row="row" />
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped lang="scss">
.gl-table-wrap {
  overflow: auto;
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  background: var(--gl-background-color-default);
  box-shadow: var(--gl-shadow-sm);
}

.gl-table {
  width: 100%;
  border-collapse: collapse;
}

.gl-table th,
.gl-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--gl-border-color-default);
  text-align: left;
  vertical-align: middle;
}

.gl-table th {
  background: var(--gl-background-color-subtle);
  font-weight: 600;
  font-size: var(--gl-font-size-sm);
  color: var(--gl-text-color-subtle);
  text-transform: none;
  letter-spacing: 0;
}

.gl-table__row {
  transition: background 0.1s;

  &:hover {
    background: var(--gl-color-gray-50);
  }

  &:last-child td {
    border-bottom: none;
  }
}

.gl-table__actions-col {
  width: 1%;
  white-space: nowrap;
}

.gl-table__actions {
  white-space: nowrap;
}

.gl-table__empty {
  text-align: center;
  color: var(--gl-text-color-subtle);
  padding: var(--gl-spacing-8) !important;
}

.gl-table td:has(.gl-btn + .gl-btn),
.gl-table td:has(button + button) {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
</style>
