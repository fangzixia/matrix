<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as runsApi from '@/api/runs'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'

const route = useRoute()
const router = useRouter()
const message = ref('')
const error = ref('')
const loading = ref(false)

async function send() {
  if (!message.value.trim()) return
  error.value = ''
  loading.value = true
  try {
    const run = await runsApi.runChat(route.params.id as string, 'default', message.value)
    message.value = ''
    router.push({ name: 'project-run-detail', params: { id: route.params.id, runId: run.id } })
  } catch (e) {
    error.value = e instanceof Error ? e.message : '发送失败'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await runsApi.listChatSessions(route.params.id as string)
})
</script>

<template>
  <div class="stack">
    <h2>AI 对话</h2>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <div class="panel stack">
      <textarea v-model="message" class="gl-textarea" rows="6" placeholder="输入消息…" />
      <GlButton variant="primary" :disabled="loading" @click="send">发送</GlButton>
    </div>
  </div>
</template>
