<template>
  <div class="auth-layout">
    <header class="auth-layout__header">
      <GlLogo />
    </header>
    <main class="auth-layout__main">
      <div class="auth-layout__grid">
        <section class="auth-layout__hero">
          <div class="auth-layout__hero-badge">自托管</div>
          <h1>完整的 AI 交付平台</h1>
          <p class="auth-layout__lead">
            Matrix 将需求、源码、AI 运行与评测整合于单一应用，覆盖从规划到验证的全流程。
          </p>
          <p class="auth-layout__hint muted">这是您自托管的 Matrix 实例。</p>
        </section>
        <section class="auth-layout__card">
          <slot />
        </section>
      </div>
    </main>
    <footer class="auth-layout__footer muted">
      Matrix
    </footer>
  </div>
</template>

<script setup lang="ts">
import GlLogo from '@/components/ui/GlLogo.vue'
</script>

<style scoped lang="scss">
.auth-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--gl-background-color-page);
}

.auth-layout__header {
  height: calc(var(--gl-header-height) + 8px);
  display: flex;
  align-items: center;
  padding: 0 var(--gl-spacing-6);
  background: var(--gl-background-color-default);
  border-bottom: 1px solid var(--gl-border-color-default);
}

.auth-layout__main {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: var(--gl-spacing-8) var(--gl-spacing-4);
}

.auth-layout__grid {
  display: grid;
  grid-template-columns: minmax(300px, 1fr) minmax(340px, 400px);
  gap: var(--gl-spacing-8);
  width: min(960px, 100%);
  align-items: center;
}

.auth-layout__hero {
  padding-right: var(--gl-spacing-4);

  h1 {
    font-size: 28px;
    line-height: 1.25;
    margin-bottom: var(--gl-spacing-3);
    color: var(--gl-text-color-default);
  }
}

.auth-layout__hero-badge {
  display: inline-block;
  padding: 4px 10px;
  margin-bottom: var(--gl-spacing-3);
  border-radius: 12px;
  background: var(--gl-color-orange-50);
  color: var(--gl-color-orange-700);
  font-size: var(--gl-font-size-sm);
  font-weight: 600;
}

.auth-layout__lead {
  line-height: 1.65;
  color: var(--gl-text-color-subtle);
  margin: 0 0 var(--gl-spacing-3);
  font-size: var(--gl-font-size-md);
}

.auth-layout__hint {
  font-size: var(--gl-font-size-sm);
  margin: 0;
}

.auth-layout__card {
  background: var(--gl-background-color-default);
  border: 1px solid var(--gl-border-color-default);
  border-radius: var(--gl-radius-lg);
  padding: var(--gl-spacing-6);
  box-shadow: var(--gl-shadow-lg);
}

.auth-layout__footer {
  text-align: center;
  padding: var(--gl-spacing-4);
  font-size: var(--gl-font-size-sm);
}

@media (max-width: 768px) {
  .auth-layout__grid {
    grid-template-columns: 1fr;
    gap: var(--gl-spacing-6);
  }

  .auth-layout__hero {
    text-align: center;
    padding-right: 0;
  }
}
</style>
