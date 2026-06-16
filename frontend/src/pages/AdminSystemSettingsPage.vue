<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import {
  getSystemSettings,
  saveSystemSettings,
  saveMcpSettings,
  testGitAccess,
  type GitAccess,
  type ModelProfile,
  type SystemSettings,
} from '@/api/system'
import { parseMcpServersJson } from '@/utils/mcpSettings'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'
import GlForm from '@/components/ui/GlForm.vue'

const auth = useAuthStore()
const error = ref('')
const message = ref('')
const saving = ref(false)
const activeTab = ref('model')
const mcpJson = ref('{}')
const stagesText = ref('')
const gitTestUrl = ref('')
const gitTestMsg = ref('')
const gitTestError = ref('')
const gitTesting = ref(false)

const tabs = [
  { key: 'model', label: '模型' },
  { key: 'mcp', label: 'MCP 服务' },
  { key: 'git', label: 'Git 访问' },
  { key: 'worker', label: '并发控制' },
  { key: 'pipeline', label: '流水线' },
]

const form = ref<SystemSettings>({
  models: [],
  context: { auto_compact_threshold: 100000, keep_recent_messages: 8 },
  security: { allow_shell: false, allow_command_mcp: false, shell_timeout: '60s' },
  mcp_servers: {},
  git: { clone_timeout: '300s', accesses: [] },
  worker: { enabled: true, poll_interval: '2s', max_attempts: 3, concurrency: 2 },
  pipeline: { default_stages: ['spec', 'implement', 'verify', 'build'], pull_before_stage: true },
})

const apiKeyPlaceholder = (m: ModelProfile) =>
  m.api_key_set ? '已配置，留空则不修改' : '未配置'

const sshKeyPlaceholder = computed(
  () => form.value.git.default_ssh_key_path || '~/.ssh/id_rsa',
)

const gitPlatformHint = computed(() => {
  const label = form.value.git.platform_label || '当前系统'
  const path = form.value.git.default_ssh_key_path
  if (!path) return `服务运行于 ${label}，请填写私钥绝对路径`
  return `服务运行于 ${label}，默认私钥路径：${path}`
})

function defaultSSHKeyPath() {
  return form.value.git.default_ssh_key_path || ''
}

async function load() {
  error.value = ''
  const s = await getSystemSettings()
  form.value = s
  if (!form.value.models?.length) {
    form.value.models = []
  }
  if (!form.value.git.accesses?.length) {
    form.value.git.accesses = []
  }
  mcpJson.value = JSON.stringify(s.mcp_servers || {}, null, 2)
  stagesText.value = (s.pipeline.default_stages || []).join(', ')
}

function newModel(): ModelProfile {
  const isFirst = form.value.models.length === 0
  return {
    id: crypto.randomUUID(),
    name: '',
    base_url: 'https://api.deepseek.com',
    model: '',
    max_tokens: 8192,
    enabled: true,
    default: isFirst,
  }
}

function addModel() {
  form.value.models.push(newModel())
}

function removeModel(index: number) {
  const wasDefault = form.value.models[index]?.default
  form.value.models.splice(index, 1)
  if (wasDefault && form.value.models.length > 0) {
    const firstEnabled = form.value.models.find((m) => m.enabled)
    if (firstEnabled) firstEnabled.default = true
    else form.value.models[0].default = true
  }
}

function setDefaultModel(id: string) {
  for (const m of form.value.models) {
    m.default = m.id === id
    if (m.id === id) {
      m.enabled = true
    }
  }
}

function newGitAccess(): GitAccess {
  return {
    id: crypto.randomUUID(),
    name: '',
    host: '*',
    ssh_key_path: defaultSSHKeyPath(),
  }
}

function addGitAccess() {
  form.value.git.accesses.push(newGitAccess())
}

function removeGitAccess(index: number) {
  form.value.git.accesses.splice(index, 1)
}

function buildPayload(): SystemSettings {
  const mcp_servers = parseMcpServersJson(mcpJson.value || '{}')
  const stages = stagesText.value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  return {
    ...form.value,
    mcp_servers,
    pipeline: { ...form.value.pipeline, default_stages: stages },
  }
}

async function saveMcp() {
  error.value = ''
  message.value = ''
  saving.value = true
  try {
    const mcp_servers = parseMcpServersJson(mcpJson.value || '{}')
    const saved = await saveMcpSettings(mcp_servers)
    form.value = saved
    mcpJson.value = JSON.stringify(saved.mcp_servers || {}, null, 2)
    message.value = 'MCP 配置已保存'
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'MCP 保存失败'
  } finally {
    saving.value = false
  }
}

async function runGitTest() {
  gitTestError.value = ''
  gitTestMsg.value = ''
  gitTesting.value = true
  try {
    const payload = buildPayload()
    await saveSystemSettings(payload)
    const res = await testGitAccess(gitTestUrl.value.trim())
    gitTestMsg.value = res.message
  } catch (e) {
    gitTestError.value = e instanceof Error ? e.message : '测试失败'
  } finally {
    gitTesting.value = false
  }
}

async function save() {
  error.value = ''
  message.value = ''
  saving.value = true
  try {
    const payload = buildPayload()
    const saved = await saveSystemSettings(payload)
    form.value = saved
    if (!form.value.models?.length) {
      form.value.models = []
    }
    if (!form.value.git.accesses?.length) {
      form.value.git.accesses = []
    }
    mcpJson.value = JSON.stringify(saved.mcp_servers || {}, null, 2)
    stagesText.value = (saved.pipeline.default_stages || []).join(', ')
    message.value = '已保存。Worker 并发数变更需重启服务后完全生效。'
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-if="auth.user?.is_root">
    <h1>系统配置</h1>
    <p class="muted">全局模型、MCP、Git、任务队列与流水线默认项。仅 root 用户可修改。</p>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <GlAlert v-if="message" variant="success">{{ message }}</GlAlert>

    <nav class="sys-tabs">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        class="sys-tabs__item"
        :class="{ 'sys-tabs__item--active': activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </nav>

    <form class="panel stack" @submit.prevent="save">
      <section v-show="activeTab === 'model'" class="stack">
        <h2>模型配置</h2>
        <p class="muted">可配置多个模型并选择启用；标记为「默认」的已启用模型将用于系统 Run。</p>
        <div class="model-list">
          <div v-for="(row, index) in form.models" :key="row.id" class="model-row panel panel--flat">
            <div class="model-row__head">
              <label class="checkbox-row">
                <input v-model="row.enabled" type="checkbox" /> 启用
              </label>
              <label class="checkbox-row">
                <input
                  :checked="row.default"
                  type="radio"
                  name="default-model"
                  @change="setDefaultModel(row.id)"
                />
                默认
              </label>
              <GlButton type="button" variant="danger" @click="removeModel(index)">删除</GlButton>
            </div>
            <GlForm label="显示名称">
              <input v-model="row.name" class="gl-input" placeholder="DeepSeek V4" />
            </GlForm>
            <GlForm label="API 地址">
              <input v-model="row.base_url" class="gl-input" placeholder="https://api.deepseek.com" />
            </GlForm>
            <GlForm label="API Key">
              <input
                v-model="row.api_key"
                class="gl-input"
                type="password"
                :placeholder="apiKeyPlaceholder(row)"
                autocomplete="new-password"
              />
            </GlForm>
            <GlForm label="模型名称">
              <input v-model="row.model" class="gl-input" placeholder="deepseek-chat" />
            </GlForm>
            <GlForm label="最大 Token">
              <input v-model.number="row.max_tokens" class="gl-input" type="number" min="1" />
            </GlForm>
          </div>
        </div>
        <GlButton type="button" @click="addModel">添加模型</GlButton>
        <h3>上下文</h3>
        <GlForm label="自动压缩阈值">
          <input v-model.number="form.context.auto_compact_threshold" class="gl-input" type="number" min="1" />
        </GlForm>
        <GlForm label="保留最近消息数">
          <input v-model.number="form.context.keep_recent_messages" class="gl-input" type="number" min="1" />
        </GlForm>
        <h3>安全</h3>
        <GlForm label="允许 Shell">
          <label class="checkbox-row"><input v-model="form.security.allow_shell" type="checkbox" /> 启用</label>
        </GlForm>
        <GlForm label="允许命令型 MCP">
          <label class="checkbox-row"><input v-model="form.security.allow_command_mcp" type="checkbox" /> 启用</label>
        </GlForm>
        <GlForm label="Shell 超时">
          <input v-model="form.security.shell_timeout" class="gl-input" placeholder="60s" />
        </GlForm>
      </section>

      <section v-show="activeTab === 'mcp'" class="stack">
        <h2>MCP 服务</h2>
        <p class="muted">
          支持直接粘贴 Cursor 的 <code>mcp.json</code>（含 <code>mcpServers</code> 包装）或裸服务对象 JSON。
          使用 command 型 MCP 时，请在「模型」页开启「允许命令型 MCP」。
        </p>
        <GlForm label="MCP 服务配置">
          <textarea v-model="mcpJson" class="gl-textarea" rows="16" spellcheck="false" />
        </GlForm>
        <GlButton type="button" variant="primary" :disabled="saving" @click="saveMcp">
          {{ saving ? '保存中…' : '保存 MCP 配置' }}
        </GlButton>
      </section>

      <section v-show="activeTab === 'git'" class="stack">
        <h2>Git 访问</h2>
        <p class="muted">按 Git 主机配置 SSH 私钥，支持多条。主机填 <code>github.com</code>、<code>gitlab.com</code> 或 <code>*</code>（默认）。</p>
        <p class="muted git-platform-hint">{{ gitPlatformHint }}</p>
        <GlForm label="克隆超时">
          <input v-model="form.git.clone_timeout" class="gl-input" placeholder="300s" />
        </GlForm>
        <div class="git-access-list">
          <div v-for="(row, index) in form.git.accesses" :key="row.id" class="git-access-row panel panel--flat">
            <GlForm label="名称">
              <input v-model="row.name" class="gl-input" placeholder="公司 GitLab" />
            </GlForm>
            <GlForm label="主机">
              <input v-model="row.host" class="gl-input" placeholder="gitlab.example.com 或 *" />
            </GlForm>
            <GlForm label="SSH 私钥路径">
              <input
                v-model="row.ssh_key_path"
                class="gl-input"
                :placeholder="sshKeyPlaceholder"
              />
            </GlForm>
            <GlButton type="button" variant="danger" @click="removeGitAccess(index)">删除</GlButton>
          </div>
        </div>
        <GlButton type="button" @click="addGitAccess">添加 Git 访问配置</GlButton>
        <div class="git-test panel panel--flat stack">
          <h3>连接测试</h3>
          <GlForm label="仓库地址">
            <input v-model="gitTestUrl" class="gl-input" placeholder="git@github.com:org/repo.git" />
          </GlForm>
          <GlButton type="button" :disabled="gitTesting || !gitTestUrl.trim()" @click="runGitTest">
            {{ gitTesting ? '测试中…' : '测试连接' }}
          </GlButton>
          <GlAlert v-if="gitTestError" variant="danger">{{ gitTestError }}</GlAlert>
          <GlAlert v-if="gitTestMsg" variant="success">{{ gitTestMsg }}</GlAlert>
          <p class="muted">测试前会先保存当前 Git 配置。</p>
        </div>
      </section>

      <section v-show="activeTab === 'worker'" class="stack">
        <h2>并发控制</h2>
        <p class="muted">控制嵌入式任务 Worker 的轮询与并发。修改并发数后建议重启 Web 服务。</p>
        <GlForm label="启用 Worker">
          <label class="checkbox-row"><input v-model="form.worker.enabled" type="checkbox" /> 启用</label>
        </GlForm>
        <GlForm label="轮询间隔">
          <input v-model="form.worker.poll_interval" class="gl-input" placeholder="2s" />
        </GlForm>
        <GlForm label="最大重试次数">
          <input v-model.number="form.worker.max_attempts" class="gl-input" type="number" min="1" />
        </GlForm>
        <GlForm label="并发数">
          <input v-model.number="form.worker.concurrency" class="gl-input" type="number" min="1" />
        </GlForm>
      </section>

      <section v-show="activeTab === 'pipeline'" class="stack">
        <h2>流水线</h2>
        <GlForm label="默认阶段（逗号分隔）">
          <input v-model="stagesText" class="gl-input" placeholder="spec, implement, verify, build" />
        </GlForm>
        <GlForm label="阶段前拉取代码">
          <label class="checkbox-row">
            <input v-model="form.pipeline.pull_before_stage" type="checkbox" /> 启用
          </label>
        </GlForm>
      </section>

      <GlButton variant="primary" type="submit" :disabled="saving">
        {{ saving ? '保存中…' : '保存配置' }}
      </GlButton>
    </form>
  </div>
  <GlAlert v-else variant="danger">仅 root 用户可访问系统配置。</GlAlert>
</template>

<style scoped lang="scss">
h1 {
  margin: 0 0 8px;
  font-size: 1.35rem;
}
h2 {
  margin: 0 0 12px;
  font-size: 1.1rem;
}
h3 {
  margin: 16px 0 8px;
  font-size: 1rem;
  color: var(--gl-text-muted);
}
.muted {
  color: var(--gl-text-muted);
  margin: 0 0 16px;
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.panel {
  margin-top: 16px;
}
.checkbox-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.sys-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--gl-border-color-default);
  margin-bottom: 16px;
}
.sys-tabs__item {
  padding: 10px 14px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--gl-text-color-subtle);
  cursor: pointer;
  font: inherit;
}
.sys-tabs__item:hover {
  color: var(--gl-text-color-default);
}
.sys-tabs__item--active {
  color: var(--gl-text-color-default);
  border-bottom-color: var(--gl-color-blue-500, #1f75cb);
}
.git-access-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.git-access-row {
  display: grid;
  gap: 12px;
}
.git-test h3 {
  margin: 0;
}
.git-platform-hint {
  padding: 8px 12px;
  background: var(--gl-background-color-subtle);
  border-radius: var(--gl-radius);
  font-size: var(--gl-font-size-sm);
}
.model-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.model-row {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.model-row__head {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
</style>
