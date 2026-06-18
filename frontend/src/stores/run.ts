import { create } from 'zustand'
import * as runsApi from '@/api/runs'
import type { Run } from '@/api/runs'

interface RunState {
  runs: Run[]
  current: Run | null
  loading: boolean
  streamEvents: string[]
  fetchRuns: (projectId: string) => Promise<void>
  setCurrent: (run: Run | null) => void
  appendStream: (data: unknown) => void
}

export const useRunStore = create<RunState>((set, get) => ({
  runs: [],
  current: null,
  loading: false,
  streamEvents: [],
  fetchRuns: async (projectId) => {
    set({ loading: true })
    try {
      const res = await runsApi.listRuns(projectId)
      set({ runs: res.runs })
    } finally {
      set({ loading: false })
    }
  },
  setCurrent: (run) => set({ current: run, streamEvents: [] }),
  appendStream: (data) => {
    set({ streamEvents: [...get().streamEvents, JSON.stringify(data, null, 2)] })
  },
}))
