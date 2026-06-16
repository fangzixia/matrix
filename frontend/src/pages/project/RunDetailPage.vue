<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useRunStore } from '@/stores/run'
import { subscribeRunStream } from '@/api/stream'
import * as runsApi from '@/api/runs'
import type { RunStep, RunEvent } from '@/api/runs'
import GlButton from '@/components/ui/GlButton.vue'
import GlBadge from '@/components/ui/GlBadge.vue'
import { runKindLabels, runStatusLabels } from '@/locales/zh-CN'

const route = useRoute()
const runStore = useRunStore()
const steps = ref<RunStep[]>([])
const events = ref<RunEvent[]>([])
let unsubscribe: (() => void) | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null

const projectId = () => route.params.id as string
const runId = () => route.params.runId as string

async function loadEvents(afterId?: string) {
  const res = await runsApi.listRunEvents(projectId(), runId(), afterId)
  if (res.events.length) {
    events.value.push(...res.events)
  }
  return res.events.at(-1)?.id
}

async function load() {
  const run = await runsApi.getRun(projectId(), runId())
  runStore.setCurrent(run)
  const stepsRes = await runsApi.listRunSteps(projectId(), runId())
  steps.value = stepsRes.steps
  events.value = []
  await loadEvents()

  const active = run.status === 'running' || run.status === 'queued' || run.status === 'pending'
  if (active) {
    unsubscribe = subscribeRunStream(projectId(), runId(), (msg) => {
      runStore.appendStream(msg)
    })
    pollTimer = setInterval(async () => {
      const lastId = events.value.at(-1)?.id
      await loadEvents(lastId)
      const updated = await runsApi.getRun(projectId(), runId())
      runStore.setCurrent(updated)
      if (!['running', 'queued', 'pending'].includes(updated.status)) {
        clearInterval(pollTimer!)
        pollTimer = null
        const stepsRes2 = await runsApi.listRunSteps(projectId(), runId())
        steps.value = stepsRes2.steps
      }
    }, 3000)
  }
}

async function cancel() {
  await runsApi.cancelRun(projectId(), runId())
  await load()
}

function statusVariant(status: string) {
  if (status === 'succeeded') return 'success'
  if (status === 'failed' || status === 'cancelled') return 'danger'
  if (status === 'running') return 'warning'
  if (status === 'queued') return 'neutral'
  return 'neutral'
}

onMounted(load)
onUnmounted(() => {
  unsubscribe?.()
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="stack">
    <div class="flex-between">
      <div>
        <h2>{{ runStore.current?.title || runId() }}</h2>
        <GlBadge v-if="runStore.current" :variant="statusVariant(runStore.current.status)">
          {{ runStatusLabels[runStore.current.status] || runStore.current.status }}
        </GlBadge>
        <p v-if="runStore.current?.error_message" class="error">{{ runStore.current.error_message }}</p>
      </div>
      <GlButton
        v-if="runStore.current?.status === 'running' || runStore.current?.status === 'queued'"
        variant="danger"
        @click="cancel"
      >取消</GlButton>
    </div>

    <div v-if="steps.length" class="panel">
      <h3>流水线步骤</h3>
      <ol class="steps">
        <li v-for="s in steps" :key="s.id">
          <GlBadge :variant="statusVariant(s.status)">{{ runKindLabels[s.kind] || s.kind }}</GlBadge>
          <span class="muted">{{ runStatusLabels[s.status] || s.status }}</span>
          <p v-if="s.output_summary" class="summary">{{ s.output_summary }}</p>
        </li>
      </ol>
    </div>

    <div class="panel stream">
      <h3>事件</h3>
      <pre v-if="events.length">{{ events.map(e => e.event_type + ': ' + (e.payload || '')).join('\n---\n') }}</pre>
      <pre v-else-if="runStore.streamEvents.length">{{ runStore.streamEvents.join('\n---\n') }}</pre>
      <p v-else class="muted">等待输出…</p>
    </div>
  </div>
</template>

<style scoped lang="scss">
.stream { max-height: 50vh; overflow: auto; }
pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-size: 12px; }
.steps { margin: 0; padding-left: 20px; }
.summary { font-size: 12px; color: var(--gl-color-neutral-600); }
.error { color: var(--gl-color-red-500); }
</style>
