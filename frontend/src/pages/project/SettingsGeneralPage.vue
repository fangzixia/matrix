<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/composables/useProjectPermissions'
import * as projectsApi from '@/api/projects'
import * as groupsApi from '@/api/groups'
import type { ProjectVisibility } from '@/api/projects'
import type { Group } from '@/api/groups'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'
import GlTabs from '@/components/ui/GlTabs.vue'
import GlModal from '@/components/ui/GlModal.vue'
import { settingsTabs } from '@/locales/zh-CN'

const route = useRoute()
const router = useRouter()
const project = useProjectStore()
const { canManageSettings, canDeleteProject } = useProjectPermissions(() => project.current)

const error = ref('')
const message = ref('')
const deleteOpen = ref(false)

const form = ref({ name: '', path: '', git_url: '', git_branch: 'main', visibility: 'private' as ProjectVisibility, group_id: '' as string })
const groups = ref<Group[]>([])

const tabs = computed(() => settingsTabs(route.params.id as string))

watch(
  () => project.current,
  (p) => {
    if (!p) return
    form.value = {
      name: p.name,
      path: p.path || '',
      git_url: p.git_url,
      git_branch: p.git_branch,
      visibility: p.visibility || 'private',
      group_id: p.group_id || '',
    }
  },
  { immediate: true },
)

onMounted(async () => {
  const res = await groupsApi.listGroups()
  groups.value = res.groups
})

async function save() {
  error.value = ''
  message.value = ''
  try {
    await projectsApi.updateProject(route.params.id as string, {
      ...form.value,
      group_id: form.value.group_id || null,
    })
    await project.fetchProject(route.params.id as string)
    message.value = '设置已保存。'
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  }
}

async function confirmDelete() {
  await projectsApi.deleteProject(route.params.id as string)
  deleteOpen.value = false
  router.push('/projects')
}
</script>

<template>
  <div v-if="canManageSettings">
    <GlTabs :tabs="tabs" />
    <h2>常规</h2>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <GlAlert v-if="message" variant="success">{{ message }}</GlAlert>
    <form class="panel stack settings-form" @submit.prevent="save">
      <label>项目名称<input v-model="form.name" required /></label>
      <label>项目路径<input v-model="form.path" placeholder="my-project" /></label>
      <label>
        所属组
        <select v-model="form.group_id">
          <option value="">— 无 —</option>
          <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
        </select>
      </label>
      <label>Git 地址<input v-model="form.git_url" /></label>
      <label>默认分支<input v-model="form.git_branch" /></label>
      <label>
        可见性
        <select v-model="form.visibility">
          <option value="private">私有</option>
          <option value="internal">内部</option>
          <option value="public">公开</option>
        </select>
      </label>
      <GlButton variant="primary" type="submit">保存更改</GlButton>
    </form>

    <div v-if="canDeleteProject" class="panel danger-zone">
      <h3>危险操作</h3>
      <p class="muted">删除项目后无法恢复，所有运行记录与成员关系将被清除。</p>
      <GlButton variant="danger" @click="deleteOpen = true">删除项目</GlButton>
    </div>

    <GlModal :open="deleteOpen" title="删除项目" @close="deleteOpen = false">
      <p>确定删除项目 <strong>{{ form.name }}</strong>？此操作不可撤销。</p>
      <template #footer>
        <GlButton @click="deleteOpen = false">取消</GlButton>
        <GlButton variant="danger" @click="confirmDelete">删除</GlButton>
      </template>
    </GlModal>
  </div>
  <GlAlert v-else variant="danger">您没有权限访问项目设置。</GlAlert>
</template>

<style scoped lang="scss">
.settings-form {
  max-width: 560px;
  margin-bottom: 24px;
}

.danger-zone {
  max-width: 560px;
  border-color: var(--gl-color-red-500);

  h3 {
    color: var(--gl-color-red-500);
  }
}
</style>
