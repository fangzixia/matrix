import { api } from './client'

export function resetUserPassword(userId: string, password: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${userId}/reset_password`, {
    method: 'POST',
    body: JSON.stringify({ password }),
  })
}

export function blockUser(userId: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${userId}/block`, { method: 'POST' })
}

export function unblockUser(userId: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${userId}/unblock`, { method: 'POST' })
}
