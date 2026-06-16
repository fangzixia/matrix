import { api } from './client'

export interface User {
  id: string
  username: string
  email: string
  name: string
  avatar_url?: string
  is_admin: boolean
  is_root?: boolean
  state: string
  last_sign_in_at?: string
  created_at: string
}

export function login(username: string, password: string) {
  return api<User>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

export function logout() {
  return api<{ ok: boolean }>('/api/auth/logout', { method: 'POST' })
}

export function me() {
  return api<User>('/api/auth/me')
}

export function updateProfile(body: { email?: string; name?: string; password?: string }) {
  return api<User>('/api/profile', {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}
