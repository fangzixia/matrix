/**
 * 认证与个人资料 API：登录、登出、当前用户、资料更新。
 */
import { api } from './client'

/** 当前登录用户 */
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

/** 登录名密码登录，成功后 Set-Cookie */
export function login(username: string, password: string) {
  return api<User>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
}

/** 注销当前 Session */
export function logout() {
  return api<{ ok: boolean }>('/api/auth/logout', { method: 'POST' })
}

/** 获取当前登录用户（未登录则 401） */
export function me() {
  return api<User>('/api/auth/me')
}

/** 更新当前用户邮箱、显示名称或密码 */
export function updateProfile(body: { email?: string; name?: string; password?: string }) {
  return api<User>('/api/profile', {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}
