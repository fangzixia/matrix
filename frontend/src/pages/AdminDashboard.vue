<template>
  <div>
    <h1 class="page-title">管理概览</h1>
    <div class="panel">
      <p class="admin-intro">
        欢迎使用 Matrix 管理区域。在此管理用户、查看实例设置并监控平台访问。
      </p>
      <router-link to="/admin/users" class="admin-link">前往用户管理 →</router-link>
    </div>
  </div>
</template>

<style scoped lang="scss">
.admin-intro {
  margin: 0 0 var(--gl-spacing-3);
  line-height: 1.6;
  color: var(--gl-text-color-subtle);
}

.admin-link {
  font-weight: 600;
  font-size: var(--gl-font-size-sm);
}
</style>
