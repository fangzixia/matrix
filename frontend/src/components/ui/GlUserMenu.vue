<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import GlAvatar from '@/components/ui/GlAvatar.vue'

defineProps<{
  name?: string
  username?: string
  isAdmin?: boolean
}>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const router = useRouter()

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) {
    close()
  }
}

onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div ref="root" class="user-menu">
    <button type="button" class="user-menu__trigger" @click.stop="toggle">
      <GlAvatar :name="name || username" :size="26" />
      <svg class="user-menu__chevron" width="12" height="12" viewBox="0 0 16 16" aria-hidden="true">
        <path fill="currentColor" d="M4 6l4 4 4-4H4z" />
      </svg>
    </button>
    <div v-if="open" class="user-menu__dropdown">
      <div class="user-menu__header">
        <GlAvatar :name="name || username" :size="40" />
        <div>
          <div class="user-menu__display">{{ name || username }}</div>
          <div class="user-menu__username muted">@{{ username }}</div>
        </div>
      </div>
      <div class="user-menu__divider" />
      <router-link to="/profile" @click="close">编辑资料</router-link>
      <router-link v-if="isAdmin" to="/admin" @click="close">管理区域</router-link>
      <div class="user-menu__divider" />
      <button type="button" class="user-menu__signout" @click="router.push('/users/sign_out')">
        退出登录
      </button>
    </div>
  </div>
</template>

<style scoped lang="scss">
.user-menu {
  position: relative;
}

.user-menu__trigger {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 6px;
  border: 1px solid transparent;
  background: transparent;
  border-radius: var(--gl-radius);
  cursor: pointer;

  &:hover {
    background: var(--gl-background-color-subtle);
    border-color: var(--gl-border-color-default);
  }
}

.user-menu__chevron {
  opacity: 0.55;
  color: var(--gl-text-color-subtle);
}

.user-menu__dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 260px;
  background: var(--gl-background-color-default);
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  box-shadow: var(--gl-shadow-dropdown);
  z-index: 400;
  overflow: hidden;
}

.user-menu__header {
  display: flex;
  gap: var(--gl-spacing-3);
  padding: var(--gl-spacing-3) var(--gl-spacing-4);
  align-items: center;
}

.user-menu__display {
  font-weight: 600;
  font-size: var(--gl-font-size-sm);
}

.user-menu__username {
  font-size: var(--gl-font-size-sm);
}

.user-menu__divider {
  height: 1px;
  background: var(--gl-border-color-default);
  margin: var(--gl-spacing-1) 0;
}

.user-menu__dropdown a,
.user-menu__signout {
  display: block;
  width: 100%;
  padding: var(--gl-spacing-2) var(--gl-spacing-4);
  text-align: left;
  color: var(--gl-text-color-default);
  text-decoration: none;
  border: none;
  background: none;
  font: inherit;
  font-size: var(--gl-font-size-sm);
  cursor: pointer;

  &:hover {
    background: var(--gl-color-blue-50);
    color: var(--gl-color-blue-600);
    text-decoration: none;
  }
}
</style>
