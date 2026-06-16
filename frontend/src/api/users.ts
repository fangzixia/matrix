import { api } from './client'
import type { User } from './auth'

export interface UserWithStats extends User {
  project_count: number
}

export interface CreateUserInput {
  username: string
  email: string
  password: string
  name: string
  is_admin?: boolean
}

export interface UpdateUserInput {
  email?: string
  name?: string
  is_admin?: boolean
  state?: string
  password?: string
}

export function searchUsers(q: string) {
  return api<{ users: User[] }>(`/api/users/search?q=${encodeURIComponent(q)}`)
}

export function listUsers() {
  return api<{ users: UserWithStats[]; total: number }>('/api/admin/users')
}

export function getUser(id: string) {
  return api<User>(`/api/admin/users/${id}`)
}

export function createUser(input: CreateUserInput) {
  return api<User>('/api/admin/users', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateUser(id: string, input: UpdateUserInput) {
  return api<User>(`/api/admin/users/${id}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteUser(id: string) {
  return api<{ ok: boolean }>(`/api/admin/users/${id}`, { method: 'DELETE' })
}
