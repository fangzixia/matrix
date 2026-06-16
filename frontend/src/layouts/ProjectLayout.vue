<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useProjectStore } from '@/stores/project'

const route = useRoute()
const project = useProjectStore()

async function load() {
  const id = route.params.id as string
  if (id) await project.fetchProject(id)
}

onMounted(load)
watch(() => route.params.id, load)
</script>

<template>
  <div class="project-layout">
    <router-view />
  </div>
</template>

<style scoped lang="scss">
.project-layout {
  min-height: 100%;
}
</style>
