<template>
  <div class="panel not-found">
    <h1 class="page-title">404</h1>
    <p class="muted">页面不存在。</p>
    <router-link to="/projects">返回项目列表</router-link>
  </div>
</template>

<style scoped lang="scss">
.not-found {
  max-width: 480px;
  margin: 48px auto;
  text-align: center;
}
</style>
