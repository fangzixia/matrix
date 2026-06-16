import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as runsApi from '@/api/runs'
import type { Run } from '@/api/runs'

export const useRunStore = defineStore('run', () => {
  const runs = ref<Run[]>([])
  const current = ref<Run | null>(null)
  const loading = ref(false)
  const streamEvents = ref<string[]>([])

  async function fetchRuns(projectId: string) {
    loading.value = true
    try {
      const res = await runsApi.listRuns(projectId)
      runs.value = res.runs
    } finally {
      loading.value = false
    }
  }

  function setCurrent(run: Run | null) {
    current.value = run
    streamEvents.value = []
  }

  function appendStream(data: unknown) {
    streamEvents.value.push(JSON.stringify(data, null, 2))
  }

  return { runs, current, loading, streamEvents, fetchRuns, setCurrent, appendStream }
})
