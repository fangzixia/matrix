import { api } from './client'
import type { MemberRole } from './projects'

export interface Group {
  id: string
  name: string
  path: string
  visibility: string
  owner_id: string
  created_at: string
  updated_at: string
}

export interface GroupMember {
  user_id: string
  username: string
  name: string
  email: string
  role: MemberRole
  created_at: string
}

export function listGroups() {
  return api<{ groups: Group[] }>('/api/groups')
}

export function createGroup(body: { name: string; path?: string; visibility?: string }) {
  return api<Group>('/api/groups', { method: 'POST', body: JSON.stringify(body) })
}

export function getGroup(id: string) {
  return api<Group>(`/api/groups/${id}`)
}

export function listGroupMembers(groupId: string) {
  return api<{ members: GroupMember[] }>(`/api/groups/${groupId}/members`)
}

export function addGroupMember(groupId: string, body: { user_id: string; role: MemberRole }) {
  return api<{ ok: boolean }>(`/api/groups/${groupId}/members`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateGroupMember(groupId: string, userId: string, role: MemberRole) {
  return api<{ ok: boolean }>(`/api/groups/${groupId}/members/${userId}`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  })
}

export function removeGroupMember(groupId: string, userId: string) {
  return api<{ ok: boolean }>(`/api/groups/${groupId}/members/${userId}`, { method: 'DELETE' })
}
