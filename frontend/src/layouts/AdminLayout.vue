<template>
  <div class="admin-layout">
    <header class="admin-layout__header">
      <router-link to="/projects" class="admin-layout__back">
        <svg width="14" height="14" viewBox="0 0 16 16" aria-hidden="true">
          <path fill="currentColor" d="M10 3L5 8l5 5V3z" />
        </svg>
        返回应用
      </router-link>
      <GlLogo :show-text="false" :size="20" />
      <span class="admin-layout__title">管理区域</span>
    </header>
    <div class="admin-layout__body">
      <aside class="admin-layout__sidebar">
        <p class="admin-layout__section">管理</p>
        <nav class="admin-layout__nav">
          <router-link to="/admin">
            <GlNavIcon name="dashboard" />
            概览
          </router-link>
          <router-link to="/admin/users">
            <GlNavIcon name="users" />
            用户
          </router-link>
          <router-link v-if="auth.isRoot" to="/admin/system">
            <GlNavIcon name="settings" />
            系统配置
          </router-link>
        </nav>
      </aside>
      <section class="admin-layout__content">
        <router-view />
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import GlLogo from '@/components/ui/GlLogo.vue'
import GlNavIcon from '@/components/ui/GlNavIcon.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<style scoped lang="scss">
.admin-layout {
  min-height: 100vh;
  background: var(--gl-background-color-page);
}

.admin-layout__header {
  height: var(--gl-header-height);
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-3);
  padding: 0 var(--gl-spacing-4);
  background: var(--gl-background-color-default);
  border-bottom: 1px solid var(--gl-border-color-default);
  box-shadow: var(--gl-shadow-sm);
}

.admin-layout__back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--gl-text-color-link);
  font-size: var(--gl-font-size-sm);
  font-weight: 600;
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}

.admin-layout__title {
  font-weight: 600;
  font-size: var(--gl-font-size-md);
  color: var(--gl-text-color-default);
  padding-left: var(--gl-spacing-2);
  border-left: 1px solid var(--gl-border-color-default);
}

.admin-layout__body {
  display: grid;
  grid-template-columns: var(--gl-sidebar-width) 1fr;
  gap: 0;
  min-height: calc(100vh - var(--gl-header-height));
}

.admin-layout__sidebar {
  background: var(--gl-background-color-default);
  border-right: 1px solid var(--gl-border-color-default);
  padding: var(--gl-spacing-4) 0;
}

.admin-layout__section {
  padding: 0 var(--gl-spacing-4);
  margin: 0 0 var(--gl-spacing-2);
  font-size: var(--gl-font-size-sm);
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--gl-text-color-subtle);
}

.admin-layout__nav {
  display: flex;
  flex-direction: column;

  a {
    display: flex;
    align-items: center;
    gap: var(--gl-spacing-3);
    padding: var(--gl-spacing-2) var(--gl-spacing-4);
    color: var(--gl-text-color-default);
    font-size: var(--gl-font-size-sm);
    text-decoration: none;
    border-left: 3px solid transparent;

    &:hover {
      background: var(--gl-background-color-subtle);
      text-decoration: none;
    }

    &.router-link-active {
      background: var(--gl-color-orange-50);
      border-left-color: var(--gl-color-orange-500);
      font-weight: 600;
    }
  }
}

.admin-layout__content {
  padding: var(--gl-spacing-6);
  min-width: 0;
}
</style>
