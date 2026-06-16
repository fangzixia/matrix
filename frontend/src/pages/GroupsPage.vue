<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import * as groupsApi from '@/api/groups'
import type { Group } from '@/api/groups'
import GlButton from '@/components/ui/GlButton.vue'
import GlTable from '@/components/ui/GlTable.vue'
import GlModal from '@/components/ui/GlModal.vue'

const router = useRouter()
const groups = ref<Group[]>([])
const open = ref(false)
const name = ref('')
const path = ref('')
const error = ref('')

const columns = [
  { key: 'name', label: '名称' },
  { key: 'path', label: '路径' },
  { key: 'visibility', label: '可见性' },
]

async function load() {
  const res = await groupsApi.listGroups()
  groups.value = res.groups
}

async function create() {
  error.value = ''
  try {
    const g = await groupsApi.createGroup({ name: name.value, path: path.value || undefined })
    open.value = false
    name.value = ''
    path.value = ''
    router.push(`/groups/${g.id}/-/members`)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '创建失败'
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex-between">
      <h2>组</h2>
      <GlButton variant="primary" @click="open = true">新建组</GlButton>
    </div>
    <GlTable :columns="columns" :rows="groups as unknown as Record<string, unknown>[]">
      <template #cell-name="{ row }">
        <router-link :to="`/groups/${(row as Group).id}/-/members`">{{ (row as Group).name }}</router-link>
      </template>
    </GlTable>

    <GlModal :open="open" title="新建组" @close="open = false">
      <p v-if="error" class="error">{{ error }}</p>
      <label>名称<input v-model="name" class="gl-input" /></label>
      <label>路径（可选）<input v-model="path" class="gl-input" placeholder="my-group" /></label>
      <template #footer>
        <GlButton @click="open = false">取消</GlButton>
        <GlButton variant="primary" @click="create">创建</GlButton>
      </template>
    </GlModal>
  </div>
</template>

<style scoped lang="scss">
.error { color: var(--gl-color-red-500); }
</style>
