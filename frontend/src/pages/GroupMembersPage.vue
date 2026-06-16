<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import * as groupsApi from '@/api/groups'
import type { GroupMember } from '@/api/groups'
import type { MemberRole } from '@/api/projects'
import type { User } from '@/api/auth'
import GlTable from '@/components/ui/GlTable.vue'
import GlButton from '@/components/ui/GlButton.vue'
import GlUserSearch from '@/components/ui/GlUserSearch.vue'
import MemberRoleSelect from '@/components/ui/MemberRoleSelect.vue'

const route = useRoute()
const groupId = route.params.id as string
const members = ref<GroupMember[]>([])
const role = ref<MemberRole>('developer')
const selectedUser = ref<User | null>(null)
const groupName = ref('')

const columns = [
  { key: 'name', label: '账号' },
  { key: 'role', label: '角色' },
]

async function load() {
  const [g, m] = await Promise.all([
    groupsApi.getGroup(groupId),
    groupsApi.listGroupMembers(groupId),
  ])
  groupName.value = g.name
  members.value = m.members
}

function onSelectUser(user: User) {
  selectedUser.value = user
}

async function add() {
  if (!selectedUser.value) return
  await groupsApi.addGroupMember(groupId, { user_id: selectedUser.value.id, role: role.value })
  selectedUser.value = null
  await load()
}

async function changeRole(m: GroupMember, newRole: MemberRole) {
  await groupsApi.updateGroupMember(groupId, m.user_id, newRole)
  await load()
}

async function remove(m: GroupMember) {
  await groupsApi.removeGroupMember(groupId, m.user_id)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h2>{{ groupName }} / 成员</h2>
    <div class="panel stack invite-form">
      <GlUserSearch @select="onSelectUser" />
      <MemberRoleSelect v-model="role" />
      <GlButton variant="primary" :disabled="!selectedUser" @click="add">添加成员</GlButton>
    </div>
    <GlTable :columns="columns" :rows="members as unknown as Record<string, unknown>[]">
      <template #cell-name="{ row }">
        {{ (row as GroupMember).name || (row as GroupMember).username }}
      </template>
      <template #cell-role="{ row }">
        <div class="member-role-cell">
          <MemberRoleSelect
            :model-value="(row as GroupMember).role"
            @update:model-value="changeRole(row as GroupMember, $event)"
          />
          <GlButton variant="danger" @click="remove(row as GroupMember)">移除</GlButton>
        </div>
      </template>
    </GlTable>
  </div>
</template>

<style scoped lang="scss">
.invite-form { max-width: 480px; margin-bottom: 20px; }

.member-role-cell {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
</style>
