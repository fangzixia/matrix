<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useProjectStore } from '@/stores/project'
import * as projectsApi from '@/api/projects'
import ProjectVisibilityBadge from '@/components/ui/ProjectVisibilityBadge.vue'
import GlBadge from '@/components/ui/GlBadge.vue'

const route = useRoute()
const project = useProjectStore()
const requirements = ref<projectsApi.RequirementItem[]>([])
const evaluations = ref<projectsApi.EvaluationItem[]>([])

onMounted(async () => {
  const id = route.params.id as string
  try {
    const [req, ev] = await Promise.all([
      projectsApi.listRequirements(id),
      projectsApi.listEvaluations(id),
    ])
    requirements.value = req.requirements ?? []
    evaluations.value = ev.evaluations ?? []
  } catch {
    requirements.value = []
    evaluations.value = []
  }
})
</script>

<template>
  <div class="overview">
    <header v-if="project.current" class="overview__header">
      <h1 class="page-title">{{ project.current.name }}</h1>
      <div class="overview__meta">
        <ProjectVisibilityBadge :visibility="project.current.visibility || 'private'" />
        <GlBadge v-if="project.current.current_user_role" variant="info">
          {{ projectsApi.roleLabels[project.current.current_user_role] }}
        </GlBadge>
      </div>
    </header>

    <div class="overview__grid">
      <section class="panel overview__card">
        <h2 class="overview__card-title">项目信息</h2>
        <dl class="overview__dl">
          <dt>Git 仓库</dt>
          <dd>{{ project.current?.git_url || '—' }}</dd>
          <dt>默认分支</dt>
          <dd><code>{{ project.current?.git_branch || 'main' }}</code></dd>
        </dl>
      </section>

      <section class="panel overview__card">
        <h2 class="overview__card-title">需求文档 (.spec)</h2>
        <ul v-if="requirements.length" class="overview__list">
          <li v-for="r in requirements" :key="r.path || r.title">{{ r.title || r.path }}</li>
        </ul>
        <p v-else class="muted overview__empty">暂无需求文档。</p>
      </section>

      <section class="panel overview__card">
        <h2 class="overview__card-title">评测报告</h2>
        <ul v-if="evaluations.length" class="overview__list">
          <li v-for="e in evaluations" :key="e.path || e.title">{{ e.title || e.path }}</li>
        </ul>
        <p v-else class="muted overview__empty">暂无评测报告。</p>
      </section>
    </div>
  </div>
</template>

<style scoped lang="scss">
.overview__header {
  margin-bottom: var(--gl-spacing-5);
}

.overview__meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--gl-spacing-2);
  margin-top: var(--gl-spacing-2);
}

.overview__grid {
  display: grid;
  gap: var(--gl-spacing-4);
}

.overview__card-title {
  font-size: var(--gl-font-size-md);
  margin-bottom: var(--gl-spacing-3);
  padding-bottom: var(--gl-spacing-2);
  border-bottom: 1px solid var(--gl-border-color-default);
}

.overview__dl {
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: var(--gl-spacing-2) var(--gl-spacing-4);
  margin: 0;
  font-size: var(--gl-font-size-sm);

  dt {
    color: var(--gl-text-color-subtle);
    font-weight: 600;
  }

  dd {
    margin: 0;
  }

  code {
    padding: 2px 6px;
    background: var(--gl-background-color-subtle);
    border-radius: var(--gl-radius);
    font-size: var(--gl-font-size-sm);
  }
}

.overview__list {
  margin: 0;
  padding-left: var(--gl-spacing-5);
  font-size: var(--gl-font-size-sm);

  li + li {
    margin-top: var(--gl-spacing-1);
  }
}

.overview__empty {
  margin: 0;
  font-size: var(--gl-font-size-sm);
}
</style>
