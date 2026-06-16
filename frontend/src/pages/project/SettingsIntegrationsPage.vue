<script setup lang="ts">
import { onMounted, ref, watch, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/composables/useProjectPermissions'
import { getIntegrations, saveIntegrations, type IntegrationSettings } from '@/api/projects'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'
import GlTabs from '@/components/ui/GlTabs.vue'
import GlForm from '@/components/ui/GlForm.vue'
import { settingsTabs } from '@/locales/zh-CN'

const route = useRoute()
const project = useProjectStore()
const { canManageSettings } = useProjectPermissions(() => project.current)
const error = ref('')
const message = ref('')
const mcpJson = ref('{}')

const form = ref<NonNullable<IntegrationSettings['model']>>({
  base_url: '',
  api_key: '',
  model: '',
  max_tokens: 8192,
})

const tabs = computed(() => settingsTabs(route.params.id as string))

async function load() {
  const s = await getIntegrations(route.params.id as string)
  if (s.model) {
    form.value = { ...form.value, ...s.model }
  }
  mcpJson.value = JSON.stringify(s.mcp_servers || {}, null, 2)
}

async function save() {
  error.value = ''
  message.value = ''
  try {
    let mcp_servers = {}
    if (mcpJson.value.trim()) {
      mcp_servers = JSON.parse(mcpJson.value)
    }
    await saveIntegrations(route.params.id as string, {
      model: form.value,
      mcp_servers,
    })
    message.value = '已保存'
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  }
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div v-if="canManageSettings">
    <GlTabs :tabs="tabs" />
    <h2>集成</h2>
    <p class="muted">项目级模型与 MCP 覆盖（留空则使用系统 YAML 默认）。</p>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <GlAlert v-if="message" variant="success">{{ message }}</GlAlert>
    <form class="panel stack" @submit.prevent="save">
      <GlForm label="模型 API 地址"><input v-model="form.base_url" class="gl-input" placeholder="https://api.deepseek.com" /></GlForm>
      <GlForm label="API Key"><input v-model="form.api_key" class="gl-input" type="password" /></GlForm>
      <GlForm label="模型名称"><input v-model="form.model" class="gl-input" placeholder="deepseek-chat" /></GlForm>
      <GlForm label="最大 Token"><input v-model.number="form.max_tokens" class="gl-input" type="number" /></GlForm>
      <GlForm label="MCP 服务（JSON）">
        <textarea v-model="mcpJson" class="gl-textarea" rows="8" />
      </GlForm>
      <GlButton variant="primary" type="submit">保存</GlButton>
    </form>
  </div>
  <GlAlert v-else variant="danger">您没有权限访问集成设置。</GlAlert>
</template>
