<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import * as usersApi from '@/api/users'
import type { UserWithStats } from '@/api/users'
import { blockUser, resetUserPassword, unblockUser } from '@/api/admin'
import GlButton from '@/components/ui/GlButton.vue'
import GlTable from '@/components/ui/GlTable.vue'
import GlBadge from '@/components/ui/GlBadge.vue'
import GlModal from '@/components/ui/GlModal.vue'
import GlAvatar from '@/components/ui/GlAvatar.vue'

const router = useRouter()
const rows = ref<UserWithStats[]>([])
const filter = ref('')
const deleteTarget = ref<UserWithStats | null>(null)
const resetTarget = ref<UserWithStats | null>(null)
const newPassword = ref('')

const columns = [
  { key: 'avatar', label: '' },
  { key: 'name', label: '姓名' },
  { key: 'username', label: '用户名' },
  { key: 'email', label: '邮箱' },
  { key: 'project_count', label: '项目数' },
  { key: 'state', label: '状态' },
  { key: 'last_sign_in_at', label: '上次登录' },
]

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter(
    (u) =>
      u.username.toLowerCase().includes(q) ||
      u.email.toLowerCase().includes(q) ||
      (u.name || '').toLowerCase().includes(q),
  )
})

async function load() {
  const res = await usersApi.listUsers()
  rows.value = res.users
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  await usersApi.deleteUser(deleteTarget.value.id)
  deleteTarget.value = null
  await load()
}

async function confirmReset() {
  if (!resetTarget.value || !newPassword.value) return
  await resetUserPassword(resetTarget.value.id, newPassword.value)
  resetTarget.value = null
  newPassword.value = ''
}

async function toggleBlock(u: UserWithStats) {
  if (u.state === 'active') {
    await blockUser(u.id)
  } else {
    await unblockUser(u.id)
  }
  await load()
}

function formatSignIn(iso?: string) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="flex-between">
      <h1 class="page-title">用户</h1>
      <GlButton variant="primary" @click="router.push({ name: 'admin-user-new' })">新建用户</GlButton>
    </div>

    <div class="admin-toolbar">
      <input v-model="filter" class="gl-input admin-filter" placeholder="搜索用户…" />
      <span class="muted">共 {{ filtered.length }} 位用户</span>
    </div>

    <GlTable :columns="columns" :rows="filtered as unknown as Record<string, unknown>[]">
      <template #cell-avatar="{ row }">
        <GlAvatar :name="(row as UserWithStats).name || (row as UserWithStats).username" />
      </template>
      <template #cell-state="{ row }">
        <GlBadge :variant="(row as UserWithStats).state === 'active' ? 'success' : 'danger'">
          {{ (row as UserWithStats).state }}
        </GlBadge>
        <GlBadge v-if="(row as UserWithStats).is_admin" variant="warning" style="margin-left: 4px">
          管理员
        </GlBadge>
      </template>
      <template #cell-last_sign_in_at="{ row }">
        {{ formatSignIn((row as UserWithStats).last_sign_in_at) }}
      </template>
      <template #actions="{ row }">
        <GlButton variant="link" @click="router.push({ name: 'admin-user-edit', params: { id: (row as UserWithStats).id } })">
          编辑
        </GlButton>
        <GlButton variant="link" @click="resetTarget = row as UserWithStats">重置密码</GlButton>
        <GlButton variant="link" @click="toggleBlock(row as UserWithStats)">
          {{ (row as UserWithStats).state === 'active' ? '封禁' : '解封' }}
        </GlButton>
        <GlButton variant="link" @click="deleteTarget = row as UserWithStats">删除</GlButton>
      </template>
    </GlTable>

    <GlModal :open="!!deleteTarget" title="删除用户" @close="deleteTarget = null">
      <p>确定删除用户 <strong>{{ deleteTarget?.username }}</strong>？</p>
      <template #footer>
        <GlButton @click="deleteTarget = null">取消</GlButton>
        <GlButton variant="danger" @click="confirmDelete">删除</GlButton>
      </template>
    </GlModal>

    <GlModal :open="!!resetTarget" title="重置密码" @close="resetTarget = null">
      <label>新密码<input v-model="newPassword" type="password" class="gl-input" /></label>
      <template #footer>
        <GlButton @click="resetTarget = null">取消</GlButton>
        <GlButton variant="primary" :disabled="!newPassword" @click="confirmReset">重置</GlButton>
      </template>
    </GlModal>
  </div>
</template>

<style scoped lang="scss">
.admin-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.admin-filter {
  width: min(320px, 100%);
}
</style>
