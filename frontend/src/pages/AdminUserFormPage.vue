<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as usersApi from '@/api/users'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'

const route = useRoute()
const router = useRouter()
const isNew = computed(() => route.name === 'admin-user-new')
const error = ref('')

const form = ref({
  username: '',
  email: '',
  name: '',
  password: '',
  is_admin: false,
  state: 'active',
})

async function load() {
  if (isNew.value) return
  const u = await usersApi.getUser(route.params.id as string)
  form.value = {
    username: u.username,
    email: u.email,
    name: u.name,
    password: '',
    is_admin: u.is_admin,
    state: u.state,
  }
}

async function save() {
  error.value = ''
  try {
    if (isNew.value) {
      await usersApi.createUser({
        username: form.value.username,
        email: form.value.email,
        name: form.value.name,
        password: form.value.password,
        is_admin: form.value.is_admin,
      })
    } else {
      const body: usersApi.UpdateUserInput = {
        email: form.value.email,
        name: form.value.name,
        is_admin: form.value.is_admin,
        state: form.value.state,
      }
      if (form.value.password) body.password = form.value.password
      await usersApi.updateUser(route.params.id as string, body)
    }
    router.push({ name: 'admin-users' })
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="page-title">{{ isNew ? '新建用户' : '编辑用户' }}</h1>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <form class="panel stack" @submit.prevent="save">
      <label v-if="isNew">用户名<input v-model="form.username" required /></label>
      <label>邮箱<input v-model="form.email" type="email" required /></label>
      <label>姓名<input v-model="form.name" /></label>
      <label>{{ isNew ? '密码' : '新密码（可选）' }}
        <input v-model="form.password" type="password" :required="isNew" />
      </label>
      <label v-if="!isNew">状态
        <select v-model="form.state">
          <option value="active">正常</option>
          <option value="blocked">已封禁</option>
        </select>
      </label>
      <label class="checkbox">
        <input v-model="form.is_admin" type="checkbox" />
        <span>
          <strong>管理员</strong>
          <span class="muted">可访问管理区域及全部项目</span>
        </span>
      </label>
      <div class="flex-between">
        <GlButton @click="router.back()">取消</GlButton>
        <GlButton variant="primary" type="submit">保存</GlButton>
      </div>
    </form>
  </div>
</template>

<style scoped lang="scss">
label.checkbox {
  flex-direction: row;
  align-items: flex-start;
  gap: 8px;

  .muted {
    display: block;
    font-weight: 400;
    font-size: 12px;
  }
}
</style>
