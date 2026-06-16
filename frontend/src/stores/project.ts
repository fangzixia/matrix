import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as projectsApi from '@/api/projects'
import type { Project } from '@/api/projects'

export const useProjectStore = defineStore('project', () => {
  const projects = ref<Project[]>([])
  const current = ref<Project | null>(null)
  const loading = ref(false)

  async function fetchProjects(scope: 'yours' | 'explore' | 'starred' = 'yours') {
    loading.value = true
    try {
      const res = await projectsApi.listProjects(scope)
      projects.value = res.projects
    } finally {
      loading.value = false
    }
  }

  async function fetchProject(id: string) {
    loading.value = true
    try {
      current.value = await projectsApi.getProject(id)
    } finally {
      loading.value = false
    }
  }

  function clearCurrent() {
    current.value = null
  }

  return { projects, current, loading, fetchProjects, fetchProject, clearCurrent }
})
