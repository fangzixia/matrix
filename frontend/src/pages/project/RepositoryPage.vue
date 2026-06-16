<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import * as projectsApi from '@/api/projects'
import { pullRepository, pushRepository } from '@/api/projects'
import type { FileEntry } from '@/api/projects'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'

const route = useRoute()
const path = ref('')
const files = ref<FileEntry[]>([])
const content = ref('')
const selected = ref('')
const message = ref('')
const error = ref('')
const info = ref('')

async function loadDir(p = '') {
  path.value = p
  const res = await projectsApi.listFiles(route.params.id as string, p)
  files.value = res.files
  content.value = ''
  selected.value = ''
}

async function openFile(file: FileEntry) {
  if (file.is_dir) {
    await loadDir(file.path)
    return
  }
  selected.value = file.path
  const res = await projectsApi.readFile(route.params.id as string, file.path)
  content.value = res.content
}

async function pull() {
  error.value = ''
  info.value = ''
  try {
    await pullRepository(route.params.id as string)
    info.value = '拉取成功'
    await loadDir(path.value)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '拉取失败'
  }
}

async function push() {
  error.value = ''
  info.value = ''
  try {
    await pushRepository(route.params.id as string, message.value)
    info.value = '推送成功'
  } catch (e) {
    error.value = e instanceof Error ? e.message : '推送失败'
  }
}

onMounted(() => loadDir())
</script>

<template>
  <div>
    <div class="repo-toolbar">
      <h2>仓库</h2>
      <div class="repo-toolbar__actions">
        <input v-model="message" class="gl-input repo-toolbar__input" placeholder="提交说明" />
        <GlButton @click="pull">拉取</GlButton>
        <GlButton variant="primary" @click="push">推送</GlButton>
      </div>
    </div>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <GlAlert v-if="info" variant="success">{{ info }}</GlAlert>
    <div class="repo">
      <aside class="repo__tree panel">
        <div class="muted" style="margin-bottom: 8px">{{ path || '/' }}</div>
        <button v-if="path" type="button" class="link" @click="loadDir('')">← 根目录</button>
        <ul>
          <li v-for="f in files" :key="f.path">
            <button type="button" class="link" @click="openFile(f)">
              {{ f.is_dir ? '📁' : '📄' }} {{ f.name }}
            </button>
          </li>
        </ul>
      </aside>
      <section class="repo__content panel">
        <h3 v-if="selected">{{ selected }}</h3>
        <pre v-if="content">{{ content }}</pre>
        <p v-else class="muted">选择文件查看内容</p>
      </section>
    </div>
  </div>
</template>

<style scoped lang="scss">
.repo-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.repo-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.repo-toolbar__input {
  min-width: 200px;
  width: min(320px, 100%);
}

.repo {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 16px;
}

.repo__tree ul {
  list-style: none;
  padding: 0;
  margin: 8px 0 0;
}

.link {
  border: none;
  background: transparent;
  color: var(--gl-text-color-link);
  cursor: pointer;
  padding: 4px 0;
  text-align: left;
}

pre {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
}
</style>
