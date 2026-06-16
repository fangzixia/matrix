<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useProjectStore } from '@/stores/project'
import * as projectsApi from '@/api/projects'
import GlButton from '@/components/ui/GlButton.vue'
import GlAvatar from '@/components/ui/GlAvatar.vue'
import GlBadge from '@/components/ui/GlBadge.vue'
import ProjectVisibilityBadge from '@/components/ui/ProjectVisibilityBadge.vue'

const router = useRouter()
const store = useProjectStore()
const scope = ref<'yours' | 'explore'>('yours')
const filter = ref('')

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return store.projects
  return store.projects.filter((p) => p.name.toLowerCase().includes(q))
})

async function load() {
  await store.fetchProjects(scope.value)
}

async function switchScope(s: 'yours' | 'explore') {
  scope.value = s
  await load()
}

onMounted(load)
</script>

<template>
  <div class="projects-page">
    <header class="page-header">
      <h1 class="page-title">项目</h1>
      <GlButton variant="primary" @click="router.push('/projects/new')">新建项目</GlButton>
    </header>

    <ul class="gl-tab-nav">
      <li>
        <button type="button" :class="{ active: scope === 'yours' }" @click="switchScope('yours')">
          我的项目
        </button>
      </li>
      <li>
        <button type="button" :class="{ active: scope === 'explore' }" @click="switchScope('explore')">
          探索项目
        </button>
      </li>
    </ul>

    <div class="projects-page__toolbar">
      <div class="gl-search projects-page__search">
        <input v-model="filter" class="gl-input" placeholder="按名称筛选…" />
      </div>
    </div>

    <ul v-if="filtered.length" class="project-list">
      <li v-for="p in filtered" :key="p.id" class="project-list__item">
        <router-link :to="`/projects/${p.id}`" class="project-list__avatar">
          <GlAvatar :name="p.name" :size="48" square />
        </router-link>
        <div class="project-list__body">
          <div class="project-list__title-row">
            <h2>
              <router-link :to="`/projects/${p.id}`">{{ p.name }}</router-link>
            </h2>
            <ProjectVisibilityBadge :visibility="p.visibility || 'private'" />
            <GlBadge v-if="p.current_user_role" variant="info">
              {{ projectsApi.roleLabels[p.current_user_role] }}
            </GlBadge>
          </div>
          <p v-if="p.git_url" class="muted project-list__desc">{{ p.git_url }}</p>
          <div class="project-list__meta muted">
            更新于 {{ projectsApi.formatRelativeTime(p.updated_at) }}
          </div>
        </div>
      </li>
    </ul>
    <div v-else class="gl-empty-state">
      {{ scope === 'explore' ? '暂无可探索的内部或公开项目' : '还没有项目 — 创建第一个项目吧。' }}
    </div>
  </div>
</template>

<style scoped lang="scss">
.projects-page__toolbar {
  margin-bottom: var(--gl-spacing-4);
}

.projects-page__search {
  max-width: 360px;
}

.project-list {
  list-style: none;
  padding: 0;
  margin: 0;
  background: var(--gl-background-color-default);
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  box-shadow: var(--gl-shadow-sm);
  overflow: hidden;
}

.project-list__item {
  display: flex;
  gap: var(--gl-spacing-4);
  align-items: flex-start;
  padding: var(--gl-spacing-4) var(--gl-spacing-5);
  border-bottom: 1px solid var(--gl-border-color-default);
  transition: background 0.12s;

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: var(--gl-color-gray-50);
  }
}

.project-list__avatar {
  text-decoration: none;
  flex-shrink: 0;
}

.project-list__body {
  flex: 1;
  min-width: 0;
}

.project-list__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--gl-spacing-2);

  h2 {
    margin: 0;
    font-size: var(--gl-font-size-lg);

    a {
      color: var(--gl-text-color-default);
      font-weight: 600;
      text-decoration: none;

      &:hover {
        color: var(--gl-color-blue-500);
        text-decoration: underline;
      }
    }
  }
}

.project-list__desc {
  margin: var(--gl-spacing-1) 0 0;
  font-size: var(--gl-font-size-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-list__meta {
  margin-top: var(--gl-spacing-2);
  font-size: var(--gl-font-size-sm);
}
</style>
