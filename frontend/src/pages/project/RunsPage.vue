<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as runsApi from '@/api/runs'
import { useRunStore } from '@/stores/run'
import GlButton from '@/components/ui/GlButton.vue'
import GlTable from '@/components/ui/GlTable.vue'
import GlBadge from '@/components/ui/GlBadge.vue'
import { runKindLabels, runStatusLabels } from '@/locales/zh-CN'

const route = useRoute()
const router = useRouter()
const runStore = useRunStore()
const message = ref('')
const filePath = ref('')
const kind = ref('task')
const pipelineMessage = ref('')

const taskKinds = [
  { value: 'task', label: runKindLabels.task },
  { value: 'chat', label: runKindLabels.chat },
  { value: 'spec', label: runKindLabels.spec },
  { value: 'implement', label: runKindLabels.implement },
  { value: 'verify', label: runKindLabels.verify },
  { value: 'build', label: runKindLabels.build },
]

const columns = [
  { key: 'title', label: '标题' },
  { key: 'kind', label: '类型' },
  { key: 'status', label: '状态' },
  { key: 'created_at', label: '创建时间' },
]

async function load() {
  await runStore.fetchRuns(route.params.id as string)
}

async function startRun() {
  const run = await runsApi.startRun(route.params.id as string, message.value || '新任务', kind.value, filePath.value)
  message.value = ''
  router.push({ name: 'project-run-detail', params: { id: route.params.id, runId: run.id } })
}

async function startPipeline() {
  const run = await runsApi.startPipeline(route.params.id as string, pipelineMessage.value || '流水线运行')
  pipelineMessage.value = ''
  router.push({ name: 'project-run-detail', params: { id: route.params.id, runId: run.id } })
}

function statusVariant(status: string) {
  if (status === 'succeeded') return 'success'
  if (status === 'failed' || status === 'cancelled') return 'danger'
  if (status === 'running') return 'warning'
  if (status === 'queued') return 'neutral'
  return 'neutral'
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex-between">
      <h2>运行</h2>
    </div>
    <div class="panel stack run-form">
      <h3>单次运行</h3>
      <select v-model="kind" class="gl-select">
        <option v-for="k in taskKinds" :key="k.value" :value="k.value">{{ k.label }}</option>
      </select>
      <input v-model="message" class="gl-input" placeholder="任务描述" />
      <input v-model="filePath" class="gl-input" placeholder="规格文件路径（可选）" />
      <GlButton variant="primary" @click="startRun">启动运行</GlButton>
    </div>
    <div class="panel stack run-form">
      <h3>流水线（规格 → 实现 → 验证 → 构建）</h3>
      <input v-model="pipelineMessage" class="gl-input" placeholder="流水线描述" />
      <GlButton variant="primary" @click="startPipeline">启动流水线</GlButton>
      <p class="muted">异步执行，由 Worker 消费队列。</p>
    </div>
    <GlTable :columns="columns" :rows="runStore.runs as unknown as Record<string, unknown>[]">
      <template #cell-title="{ row }">
        <router-link :to="{ name: 'project-run-detail', params: { id: route.params.id, runId: (row as runsApi.Run).id } }">
          {{ (row as runsApi.Run).title || (row as runsApi.Run).id }}
        </router-link>
      </template>
      <template #cell-kind="{ row }">
        {{ runKindLabels[(row as runsApi.Run).kind] || (row as runsApi.Run).kind }}
      </template>
      <template #cell-status="{ row }">
        <GlBadge :variant="statusVariant((row as runsApi.Run).status)">
          {{ runStatusLabels[(row as runsApi.Run).status] || (row as runsApi.Run).status }}
        </GlBadge>
      </template>
      <template #cell-created_at="{ row }">
        {{ new Date((row as runsApi.Run).created_at).toLocaleString('zh-CN') }}
      </template>
    </GlTable>
  </div>
</template>

<style scoped lang="scss">
.run-form {
  margin-bottom: 16px;
}
</style>
