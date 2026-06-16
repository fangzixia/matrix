<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import * as reposApi from '@/api/repositories'
import type { Repository } from '@/api/repositories'
import GlButton from '@/components/ui/GlButton.vue'
import GlTable from '@/components/ui/GlTable.vue'
import GlAlert from '@/components/ui/GlAlert.vue'
import GlTabs from '@/components/ui/GlTabs.vue'
import { settingsTabs } from '@/locales/zh-CN'

const route = useRoute()
const projectId = route.params.id as string
const tabs = computed(() => settingsTabs(projectId))
const repos = ref<Repository[]>([])
const message = ref('')
const error = ref('')
const form = ref({ name: '', git_url: '', git_branch: 'main', is_default: false })

const columns = [
  { key: 'name', label: '名称' },
  { key: 'git_url', label: 'Git 地址' },
  { key: 'git_branch', label: '分支' },
  { key: 'is_default', label: '默认' },
]

async function load() {
  const res = await reposApi.listRepositories(projectId)
  repos.value = res.repositories
}

async function add() {
  error.value = ''
  try {
    await reposApi.createRepository(projectId, form.value)
    form.value = { name: '', git_url: '', git_branch: 'main', is_default: false }
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '添加失败'
  }
}

async function pull(r: Repository) {
  message.value = ''
  await reposApi.pullRepo(projectId, r.id)
  message.value = `已拉取 ${r.name}`
}

async function push(r: Repository) {
  message.value = ''
  await reposApi.pushRepo(projectId, r.id)
  message.value = `已推送 ${r.name}`
}

async function remove(r: Repository) {
  await reposApi.deleteRepository(projectId, r.id)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <GlTabs :tabs="tabs" />
    <h2>仓库</h2>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <GlAlert v-if="message" variant="success">{{ message }}</GlAlert>

    <form class="panel stack repo-form" @submit.prevent="add">
      <label>名称<input v-model="form.name" required /></label>
      <label>Git 地址<input v-model="form.git_url" /></label>
      <label>分支<input v-model="form.git_branch" /></label>
      <label><input v-model="form.is_default" type="checkbox" /> 设为默认仓库</label>
      <GlButton variant="primary" type="submit">添加仓库</GlButton>
    </form>

    <GlTable :columns="columns" :rows="repos as unknown as Record<string, unknown>[]">
      <template #cell-is_default="{ row }">{{ (row as Repository).is_default ? '是' : '' }}</template>
      <template #actions="{ row }">
        <GlButton @click="pull(row as Repository)">拉取</GlButton>
        <GlButton @click="push(row as Repository)">推送</GlButton>
        <GlButton variant="danger" @click="remove(row as Repository)">删除</GlButton>
      </template>
    </GlTable>
  </div>
</template>

<style scoped lang="scss">
.repo-form { max-width: 560px; margin-bottom: 24px; }
</style>
