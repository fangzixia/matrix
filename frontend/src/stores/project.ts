import { create } from 'zustand'
import * as projectsApi from '@/api/projects'
import type { Project } from '@/api/projects'

interface ProjectState {
  projects: Project[]
  current: Project | null
  loading: boolean
  fetchProjects: (scope?: 'yours' | 'explore' | 'starred') => Promise<void>
  fetchProject: (id: string) => Promise<void>
  clearCurrent: () => void
}

export const useProjectStore = create<ProjectState>((set) => ({
  projects: [],
  current: null,
  loading: false,
  fetchProjects: async (scope = 'yours') => {
    set({ loading: true })
    try {
      const res = await projectsApi.listProjects(scope)
      set({ projects: res.projects })
    } finally {
      set({ loading: false })
    }
  },
  fetchProject: async (id) => {
    set({ loading: true })
    try {
      const current = await projectsApi.getProject(id)
      set({ current })
    } finally {
      set({ loading: false })
    }
  },
  clearCurrent: () => set({ current: null }),
}))
