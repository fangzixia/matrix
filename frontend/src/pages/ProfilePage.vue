<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import * as authApi from '@/api/auth'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'

const auth = useAuthStore()
const form = ref({
  name: auth.user?.name || '',
  email: auth.user?.email || '',
  password: '',
})
const message = ref('')
const error = ref('')

async function save() {
  error.value = ''
  message.value = ''
  try {
    const body: { name?: string; email?: string; password?: string } = {
      name: form.value.name,
      email: form.value.email,
    }
    if (form.value.password) body.password = form.value.password
    auth.user = await authApi.updateProfile(body)
    message.value = '已保存'
  } catch (e) {
    error.value = e instanceof Error ? e.message : '保存失败'
  }
}
</script>

<template>
  <div>
    <h1 class="page-title">个人资料</h1>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <GlAlert v-if="message" variant="success">{{ message }}</GlAlert>
    <form class="panel stack" style="max-width: 480px" @submit.prevent="save">
      <label>姓名<input v-model="form.name" class="gl-input" /></label>
      <label>邮箱<input v-model="form.email" type="email" class="gl-input" /></label>
      <label>新密码<input v-model="form.password" type="password" class="gl-input" /></label>
      <GlButton variant="primary" type="submit">保存更改</GlButton>
    </form>
  </div>
</template>
