<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/composables/useProjectPermissions'
import GlDropdown from '@/components/ui/GlDropdown.vue'
import GlUserMenu from '@/components/ui/GlUserMenu.vue'
import GlLogo from '@/components/ui/GlLogo.vue'
import GlAvatar from '@/components/ui/GlAvatar.vue'
import GlNavIcon from '@/components/ui/GlNavIcon.vue'
import * as notificationsApi from '@/api/notifications'
import type { Notification } from '@/api/notifications'

const auth = useAuthStore()
const projectStore = useProjectStore()
const route = useRoute()
const router = useRouter()
const search = ref('')
const menuOpen = ref(false)
const notifyOpen = ref(false)
const unreadCount = ref(0)
const notifications = ref<Notification[]>([])

const projectId = computed(() => route.params.id as string | undefined)
const isAdminRoute = computed(() => route.matched.some((r) => r.meta.admin))
const { canManageSettings } = useProjectPermissions(() => projectStore.current)

const filteredProjects = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return projectStore.projects
  return projectStore.projects.filter((p) => p.name.toLowerCase().includes(q))
})

const navItems = computed(() => {
  if (!projectId.value) return []
  const base = `/projects/${projectId.value}`
  const items = [
    { to: base, label: '概览', icon: 'overview' as const },
    { to: `${base}/chat`, label: '对话', icon: 'chat' as const },
    { to: `${base}/runs`, label: '运行', icon: 'runs' as const },
    { to: `${base}/repository`, label: '仓库', icon: 'repository' as const },
  ]
  if (canManageSettings.value) {
    items.push({ to: `${base}/-/settings/general`, label: '设置', icon: 'settings' as const })
  }
  return items
})

onMounted(() => {
  if (!projectStore.projects.length) {
    projectStore.fetchProjects()
  }
  loadNotifications()
  setInterval(loadNotifications, 30000)
})

async function loadNotifications() {
  if (!auth.user) return
  try {
    const [countRes, listRes] = await Promise.all([
      notificationsApi.unreadCount(),
      notificationsApi.listNotifications(),
    ])
    unreadCount.value = countRes.count
    notifications.value = listRes.notifications
  } catch {
    /* ignore when logged out */
  }
}

async function markRead(n: Notification) {
  await notificationsApi.markRead(n.id)
  if (n.link) router.push(n.link)
  notifyOpen.value = false
  await loadNotifications()
}

function goProject(id: string) {
  menuOpen.value = false
  router.push(`/projects/${id}`)
}
</script>

<template>
  <div class="app-shell">
    <header class="top-bar">
      <div class="top-bar__left">
        <router-link to="/projects" class="top-bar__logo" title="工作台">
          <GlLogo />
        </router-link>

        <nav class="top-bar__nav">
          <GlDropdown v-model:open="menuOpen" label="项目" variant="nav">
            <div class="project-picker">
              <div class="gl-search project-picker__search">
                <input
                  v-model="search"
                  class="gl-input"
                  placeholder="搜索项目"
                  @click.stop
                />
              </div>
              <div class="project-picker__list">
                <button
                  v-for="p in filteredProjects"
                  :key="p.id"
                  type="button"
                  class="project-picker__item"
                  @click="goProject(p.id)"
                >
                  <GlAvatar :name="p.name" :size="28" square />
                  <span>{{ p.name }}</span>
                </button>
                <p v-if="!filteredProjects.length" class="project-picker__empty muted">未找到项目</p>
              </div>
              <button
                type="button"
                class="project-picker__new"
                @click="router.push('/projects/new'); menuOpen = false"
              >
                创建新项目
              </button>
            </div>
          </GlDropdown>
          <router-link to="/groups" class="top-bar__nav-link">组</router-link>
        </nav>

        <div v-if="projectId && projectStore.current" class="top-bar__breadcrumb">
          <span class="top-bar__sep">/</span>
          <router-link :to="`/projects/${projectId}`">{{ projectStore.current.name }}</router-link>
        </div>
      </div>

      <div class="top-bar__right">
        <div class="notify-wrap">
          <button type="button" class="notify-bell" title="通知" @click="notifyOpen = !notifyOpen">
            <svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true">
              <path fill="currentColor" d="M8 1.5a4.5 4.5 0 0 0-4.5 4.5v2.1l-.9 1.8A1 1 0 0 0 3.5 11h9a1 1 0 0 0 .9-1.1l-.9-1.8V6A4.5 4.5 0 0 0 8 1.5zm0 12.5a2 2 0 0 0 2-2H6a2 2 0 0 0 2 2z" />
            </svg>
            <span v-if="unreadCount" class="notify-badge">{{ unreadCount }}</span>
          </button>
          <div v-if="notifyOpen" class="notify-panel">
            <p v-if="!notifications.length" class="muted">暂无通知</p>
            <button
              v-for="n in notifications"
              :key="n.id"
              type="button"
              class="notify-item"
              :class="{ unread: !n.read_at }"
              @click="markRead(n)"
            >
              <strong>{{ n.title }}</strong>
              <span>{{ n.body }}</span>
            </button>
          </div>
        </div>
        <router-link to="/projects/new" class="top-bar__action" title="新建项目">
          <svg width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
            <path fill="currentColor" d="M8 2a1 1 0 0 1 1 1v4h4a1 1 0 1 1 0 2H9v4a1 1 0 1 1-2 0V9H3a1 1 0 1 1 0-2h4V3a1 1 0 0 1 1-1z" />
          </svg>
          <span class="top-bar__action-label">新建</span>
        </router-link>
        <GlUserMenu
          :name="auth.user?.name"
          :username="auth.user?.username"
          :is-admin="auth.isAdmin"
        />
      </div>
    </header>

    <div class="app-shell__body">
      <aside v-if="!isAdminRoute && projectId && projectStore.current" class="project-sidebar">
        <router-link :to="`/projects/${projectId}`" class="project-sidebar__head">
          <GlAvatar :name="projectStore.current.name" :size="32" square />
          <span class="project-sidebar__name">{{ projectStore.current.name }}</span>
        </router-link>
        <nav class="project-sidebar__nav">
          <router-link
            v-for="item in navItems"
            :key="item.to"
            :to="item.to"
            class="project-sidebar__link"
          >
            <GlNavIcon :name="item.icon" />
            <span>{{ item.label }}</span>
          </router-link>
        </nav>
      </aside>

      <main class="app-shell__main">
        <div class="app-shell__content page-container">
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>

<style scoped lang="scss">
.app-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.top-bar {
  height: var(--gl-header-height);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--gl-spacing-3);
  background: var(--gl-background-color-header);
  border-bottom: 1px solid var(--gl-border-color-default);
  box-shadow: var(--gl-shadow-sm);
  position: sticky;
  top: 0;
  z-index: 200;
}

.top-bar__left,
.top-bar__right {
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-2);
  min-width: 0;
}

.top-bar__logo {
  display: flex;
  align-items: center;
  padding: var(--gl-spacing-1) var(--gl-spacing-2);
  border-radius: var(--gl-radius);
  text-decoration: none;

  &:hover {
    background: var(--gl-background-color-subtle);
    text-decoration: none;
  }
}

.top-bar__nav {
  display: flex;
  align-items: center;
}

.top-bar__breadcrumb {
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-2);
  font-size: var(--gl-font-size-sm);
  min-width: 0;

  a {
    color: var(--gl-text-color-default);
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 240px;
    text-decoration: none;

    &:hover {
      color: var(--gl-color-blue-500);
    }
  }
}

.top-bar__sep {
  color: var(--gl-text-color-subtle);
}

.top-bar__action {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: var(--gl-radius);
  color: var(--gl-text-color-default);
  font-size: var(--gl-font-size-sm);
  font-weight: 600;
  text-decoration: none;

  &:hover {
    background: var(--gl-background-color-subtle);
    text-decoration: none;
  }
}

.top-bar__action-label {
  @media (max-width: 768px) {
    display: none;
  }
}

.project-picker {
  min-width: 280px;
  max-width: 320px;
}

.project-picker__search {
  padding: var(--gl-spacing-2);
  border-bottom: 1px solid var(--gl-border-color-default);
}

.project-picker__list {
  max-height: 280px;
  overflow-y: auto;
  padding: var(--gl-spacing-1) 0;
}

.project-picker__item {
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-2);
  width: 100%;
  padding: var(--gl-spacing-2) var(--gl-spacing-3);
  border: none;
  background: none;
  font: inherit;
  text-align: left;
  cursor: pointer;
  color: var(--gl-text-color-default);

  &:hover {
    background: var(--gl-color-blue-50);
    color: var(--gl-color-blue-600);
  }

  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.project-picker__empty {
  padding: var(--gl-spacing-3);
  margin: 0;
  font-size: var(--gl-font-size-sm);
  text-align: center;
}

.project-picker__new {
  display: block;
  width: 100%;
  padding: var(--gl-spacing-2) var(--gl-spacing-3);
  border: none;
  border-top: 1px solid var(--gl-border-color-default);
  background: var(--gl-background-color-subtle);
  font: inherit;
  font-weight: 600;
  font-size: var(--gl-font-size-sm);
  color: var(--gl-color-blue-500);
  cursor: pointer;
  text-align: left;

  &:hover {
    background: var(--gl-color-blue-50);
  }
}

.app-shell__body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.project-sidebar {
  width: var(--gl-sidebar-width);
  flex-shrink: 0;
  background: var(--gl-background-color-sidebar);
  border-right: 1px solid var(--gl-border-color-default);
  display: flex;
  flex-direction: column;
}

.project-sidebar__head {
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-2);
  padding: var(--gl-spacing-3) var(--gl-spacing-4);
  border-bottom: 1px solid var(--gl-border-color-default);
  text-decoration: none;
  color: var(--gl-text-color-default);

  &:hover {
    background: var(--gl-background-color-subtle);
    text-decoration: none;
  }
}

.project-sidebar__name {
  font-weight: 600;
  font-size: var(--gl-font-size-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-sidebar__nav {
  padding: var(--gl-spacing-2) 0;
  display: flex;
  flex-direction: column;
}

.project-sidebar__link {
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-3);
  padding: var(--gl-spacing-2) var(--gl-spacing-4);
  color: var(--gl-text-color-default);
  font-size: var(--gl-font-size-sm);
  text-decoration: none;
  border-left: 3px solid transparent;
  transition: background 0.12s, border-color 0.12s;

  &:hover {
    background: var(--gl-background-color-subtle);
    text-decoration: none;
  }

  &.router-link-active {
    background: var(--gl-color-orange-50);
    border-left-color: var(--gl-color-orange-500);
    font-weight: 600;
    color: var(--gl-text-color-default);

    :deep(.gl-nav-icon) {
      opacity: 1;
      color: var(--gl-color-orange-600);
    }
  }
}

.app-shell__main {
  flex: 1;
  min-width: 0;
  overflow: auto;
}

.app-shell__content {
  padding: var(--gl-spacing-6);
}

.top-bar__nav-link {
  font-size: var(--gl-font-size-md);
  font-weight: 600;
  color: var(--gl-text-color-default);
  text-decoration: none;
  padding: 6px 10px;
  border-radius: var(--gl-radius);

  &:hover {
    background: var(--gl-background-color-subtle);
    text-decoration: none;
  }
}

.notify-wrap {
  position: relative;
}

.notify-bell {
  position: relative;
  display: inline-flex;
  padding: 6px 10px;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: var(--gl-radius);
  color: var(--gl-text-color-default);

  &:hover {
    background: var(--gl-background-color-subtle);
  }
}

.notify-badge {
  position: absolute;
  top: 2px;
  right: 4px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  font-size: 10px;
  line-height: 16px;
  text-align: center;
  background: var(--gl-color-red-500);
  color: #fff;
  border-radius: 8px;
}

.notify-panel {
  position: absolute;
  right: 0;
  top: calc(100% + 4px);
  width: 320px;
  max-height: 360px;
  overflow: auto;
  background: var(--gl-background-color-default);
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius);
  box-shadow: var(--gl-shadow-dropdown);
  z-index: 300;
  padding: 8px;
}

.notify-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  width: 100%;
  padding: 8px;
  border: none;
  background: transparent;
  text-align: left;
  cursor: pointer;
  border-radius: var(--gl-radius);
  font: inherit;

  &:hover {
    background: var(--gl-background-color-subtle);
  }

  &.unread strong {
    color: var(--gl-color-blue-600);
  }
}
</style>
