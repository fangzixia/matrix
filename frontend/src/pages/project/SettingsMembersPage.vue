<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/composables/useProjectPermissions'
import * as projectsApi from '@/api/projects'
import type { ProjectMember, MemberRole } from '@/api/projects'
import type { User } from '@/api/auth'
import GlTable from '@/components/ui/GlTable.vue'
import GlButton from '@/components/ui/GlButton.vue'
import GlAlert from '@/components/ui/GlAlert.vue'
import GlTabs from '@/components/ui/GlTabs.vue'
import GlAvatar from '@/components/ui/GlAvatar.vue'
import GlUserSearch from '@/components/ui/GlUserSearch.vue'
import MemberRoleSelect from '@/components/ui/MemberRoleSelect.vue'
import { settingsTabs } from '@/locales/zh-CN'

const route = useRoute()
const project = useProjectStore()
const { canManageMembers } = useProjectPermissions(() => project.current)

const members = ref<ProjectMember[]>([])
const username = ref('')
const selectedUser = ref<User | null>(null)
const role = ref<MemberRole>('developer')
const error = ref('')

const tabs = computed(() => settingsTabs(route.params.id as string))

const columns = [
  { key: 'avatar', label: '' },
  { key: 'name', label: '账号' },
  { key: 'role', label: '角色' },
  { key: 'created_at', label: '加入时间' },
]

async function load() {
  const res = await projectsApi.listMembers(route.params.id as string)
  members.value = res.members
}

function onSelectUser(user: User) {
  selectedUser.value = user
}

async function invite() {
  error.value = ''
  const name = username.value.trim()
  if (!name) return
  try {
    await projectsApi.addMember(route.params.id as string, {
      username: name,
      role: role.value,
    })
    username.value = ''
    selectedUser.value = null
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : '添加失败'
  }
}

async function changeRole(m: ProjectMember, newRole: MemberRole) {
  await projectsApi.updateMember(route.params.id as string, m.user_id, newRole)
  await load()
}

async function remove(m: ProjectMember) {
  await projectsApi.removeMember(route.params.id as string, m.user_id)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <GlTabs :tabs="tabs" />
    <h2>项目成员</h2>
    <GlAlert v-if="error" variant="danger">{{ error }}</GlAlert>

    <div v-if="canManageMembers" class="panel stack invite-panel">
      <h3>邀请成员</h3>
      <p class="muted">按用户名、姓名或邮箱搜索。</p>
      <label>
        用户
        <GlUserSearch v-model="username" @select="onSelectUser" />
      </label>
      <p v-if="selectedUser" class="selected-user">
        已选择：<strong>{{ selectedUser.name || selectedUser.username }}</strong>
        (@{{ selectedUser.username }})
      </p>
      <label>
        角色
        <MemberRoleSelect v-model="role" show-hints />
      </label>
      <GlButton variant="primary" :disabled="!username.trim()" @click="invite">邀请</GlButton>
    </div>

    <GlTable :columns="columns" :rows="members as unknown as Record<string, unknown>[]">
      <template #cell-avatar="{ row }">
        <GlAvatar :name="(row as ProjectMember).name || (row as ProjectMember).username" />
      </template>
      <template #cell-name="{ row }">
        <div>{{ (row as ProjectMember).name || (row as ProjectMember).username }}</div>
        <div class="muted">@{{ (row as ProjectMember).username }}</div>
      </template>
      <template #cell-role="{ row }">
        <MemberRoleSelect
          v-if="canManageMembers"
          :model-value="(row as ProjectMember).role"
          @update:model-value="changeRole(row as ProjectMember, $event)"
        />
        <span v-else>{{ projectsApi.roleLabels[(row as ProjectMember).role] }}</span>
      </template>
      <template #cell-created_at="{ row }">
        {{ new Date((row as ProjectMember).created_at).toLocaleDateString() }}
      </template>
      <template #actions="{ row }">
        <GlButton
          v-if="canManageMembers"
          variant="link"
          @click="remove(row as ProjectMember)"
        >
          移除
        </GlButton>
      </template>
    </GlTable>
  </div>
</template>

<style scoped lang="scss">
.invite-panel {
  margin-bottom: 16px;
  max-width: 560px;
}

label {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-weight: 600;
}

.selected-user {
  font-size: 13px;
  margin: 0;
}
</style>
