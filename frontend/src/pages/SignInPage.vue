<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { resolvePostLoginRedirect } from '@/router/guards'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

const login = ref('')
const password = ref('')
const remember = ref(false)
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(login.value, password.value)
    const redirect = resolvePostLoginRedirect(route.query.redirect as string | undefined)
    router.push(redirect)
  } catch (e) {
    error.value = e instanceof Error ? e.message : '用户名或密码错误。'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="sign-in">
    <h2 class="sign-in__title">登录</h2>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>
    <form class="sign-in__form" @submit.prevent="submit">
      <div class="sign-in__field">
        <label class="gl-label" for="login">用户名或邮箱</label>
        <input id="login" v-model="login" class="gl-input" required autocomplete="username" />
      </div>
      <div class="sign-in__field">
        <label class="gl-label" for="password">密码</label>
        <input id="password" v-model="password" class="gl-input" type="password" required autocomplete="current-password" />
      </div>
      <div class="sign-in__extras">
        <label class="sign-in__remember">
          <input v-model="remember" type="checkbox" />
          记住我
        </label>
      </div>
      <GlButton variant="primary" type="submit" class="sign-in__submit" :disabled="loading">
        {{ loading ? '登录中…' : '登录' }}
      </GlButton>
    </form>
  </div>
</template>

<style scoped lang="scss">
.sign-in__title {
  font-size: var(--gl-font-size-xl);
  margin-bottom: var(--gl-spacing-5);
  font-weight: 600;
}

.sign-in__form {
  display: flex;
  flex-direction: column;
  gap: var(--gl-spacing-4);
}

.sign-in__field {
  display: flex;
  flex-direction: column;
}

.sign-in__extras {
  display: flex;
  align-items: center;
  font-size: var(--gl-font-size-sm);
}

.sign-in__remember {
  display: flex;
  align-items: center;
  gap: var(--gl-spacing-2);
  cursor: pointer;
  color: var(--gl-text-color-default);
}

.sign-in__submit {
  width: 100%;
  padding: 10px 16px;
  margin-top: var(--gl-spacing-1);
}
</style>
