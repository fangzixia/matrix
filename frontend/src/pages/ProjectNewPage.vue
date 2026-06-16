<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import * as projectsApi from '@/api/projects'
import type { ProjectVisibility } from '@/api/projects'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'

const router = useRouter()
const error = ref('')
const loading = ref(false)

const form = ref({
  name: '',
  git_url: '',
  git_branch: 'main',
  visibility: 'private' as ProjectVisibility,
})

const visibilityOptions: { value: ProjectVisibility; label: string; hint: string }[] = [
  { value: 'private', label: '私有', hint: '仅被明确授权的用户可访问。' },
  { value: 'internal', label: '内部', hint: '所有已登录用户可访问。' },
  { value: 'public', label: '公开', hint: '所有已登录用户可访问。' },
]

async function create() {
  error.value = ''
  loading.value = true
  try {
    const p = await projectsApi.createProject({
      name: form.value.name,
      git_url: form.value.git_url || undefined,
      git_branch: form.value.git_branch || undefined,
      visibility: form.value.visibility,
    })
    router.push(`/projects/${p.id}`)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '创建项目失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="new-project">
    <h1 class="page-title">创建新项目</h1>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <form class="panel stack new-project__form" @submit.prevent="create">
      <div>
        <label class="gl-label" for="name">项目名称</label>
        <input id="name" v-model="form.name" class="gl-input" required placeholder="my-awesome-project" />
      </div>
      <div>
        <label class="gl-label" for="git">Git 地址（可选）</label>
        <input id="git" v-model="form.git_url" class="gl-input" placeholder="https://gitlab.example.com/group/project.git" />
      </div>
      <div>
        <label class="gl-label" for="branch">默认分支</label>
        <input id="branch" v-model="form.git_branch" class="gl-input" placeholder="main" />
      </div>
      <fieldset class="visibility-fieldset">
        <legend class="gl-label">可见性</legend>
        <label v-for="opt in visibilityOptions" :key="opt.value" class="visibility-option">
          <input v-model="form.visibility" type="radio" :value="opt.value" />
          <span>
            <strong>{{ opt.label }}</strong>
            <span class="muted">{{ opt.hint }}</span>
          </span>
        </label>
      </fieldset>
      <div class="flex-between">
        <GlButton type="button" @click="router.push('/projects')">取消</GlButton>
        <GlButton variant="primary" type="submit" :disabled="loading || !form.name">
          {{ loading ? '创建中…' : '创建项目' }}
        </GlButton>
      </div>
    </form>
  </div>
</template>

<style scoped lang="scss">
.new-project__form {
  max-width: 560px;
}

.visibility-fieldset {
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  padding: var(--gl-spacing-3) var(--gl-spacing-4);
  margin: 0;
  background: var(--gl-background-color-subtle);

  legend {
    margin-bottom: var(--gl-spacing-2);
  }
}

.visibility-option {
  display: flex;
  flex-direction: row;
  align-items: flex-start;
  gap: var(--gl-spacing-3);
  padding: var(--gl-spacing-2) 0;
  cursor: pointer;

  strong {
    display: block;
    font-size: var(--gl-font-size-sm);
  }

  .muted {
    font-size: var(--gl-font-size-sm);
  }
}
</style>
