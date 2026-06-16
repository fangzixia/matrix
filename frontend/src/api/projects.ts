import { api } from './client'

export type ProjectVisibility = 'private' | 'internal' | 'public'

export type MemberRole = 'guest' | 'reporter' | 'developer' | 'maintainer' | 'owner'

export interface ProjectPermissions {
  read: boolean
  create_run: boolean
  manage_settings: boolean
  manage_members: boolean
  delete_project: boolean
  git_pull: boolean
  git_push: boolean
}

export interface Project {
  id: string
  name: string
  path?: string
  git_url: string
  git_branch: string
  visibility: ProjectVisibility
  group_id?: string | null
  owner_id: string
  created_at: string
  updated_at: string
  current_user_role?: MemberRole | null
  permissions?: ProjectPermissions
}

export interface ProjectMember {
  user_id: string
  username: string
  name: string
  email: string
  role: MemberRole
  created_at: string
}

export interface FileEntry {
  name: string
  path: string
  is_dir: boolean
  size?: number
}

export interface RequirementItem {
  id?: string
  path: string
  title: string
  content?: string
}

export interface EvaluationItem {
  id?: string
  kind?: string
  path: string
  title: string
}

export interface IntegrationSettings {
  model?: {
    base_url?: string
    api_key?: string
    model?: string
    max_tokens?: number
  }
  mcp_servers?: Record<string, {
    command?: string
    args?: string[]
    url?: string
    disabled?: boolean
    headers?: Record<string, string>
  }>
}

export function listProjects(scope: 'yours' | 'explore' | 'starred' = 'yours') {
  return api<{ projects: Project[] }>(`/api/projects?scope=${scope}`)
}

export function getProject(id: string) {
  return api<Project>(`/api/projects/${id}`)
}

export function createProject(body: {
  name: string
  git_url?: string
  git_branch?: string
  visibility?: ProjectVisibility
}) {
  return api<Project>('/api/projects', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateProject(id: string, body: {
  name?: string
  path?: string
  git_url?: string
  git_branch?: string
  visibility?: ProjectVisibility
  group_id?: string | null
}) {
  return api<Project>(`/api/projects/${id}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function deleteProject(id: string) {
  return api<{ ok: boolean }>(`/api/projects/${id}`, { method: 'DELETE' })
}

export function listMembers(projectId: string) {
  return api<{ members: ProjectMember[] }>(`/api/projects/${projectId}/members`)
}

export function addMember(projectId: string, body: { username?: string; user_id?: string; role: MemberRole }) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/members`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateMember(projectId: string, userId: string, role: MemberRole) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/members/${userId}`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  })
}

export function removeMember(projectId: string, userId: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/members/${userId}`, { method: 'DELETE' })
}

export function listFiles(projectId: string, path = '') {
  const q = path ? `?path=${encodeURIComponent(path)}` : ''
  return api<{ files: FileEntry[] }>(`/api/projects/${projectId}/repository/tree${q}`)
}

export function readFile(projectId: string, path: string) {
  return api<{ content: string }>(`/api/projects/${projectId}/repository/file?path=${encodeURIComponent(path)}`)
}

export function listRequirements(projectId: string) {
  return api<{ requirements: RequirementItem[] | null }>(`/api/projects/${projectId}/requirements`)
}

export function listEvaluations(projectId: string) {
  return api<{ evaluations: EvaluationItem[] | null }>(`/api/projects/${projectId}/evaluations`)
}

export function getIntegrations(projectId: string) {
  return api<IntegrationSettings>(`/api/projects/${projectId}/settings/integrations`)
}

export function saveIntegrations(projectId: string, body: IntegrationSettings) {
  return api<IntegrationSettings>(`/api/projects/${projectId}/settings/integrations`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function pullRepository(projectId: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/repository/pull`, { method: 'POST' })
}

export function pushRepository(projectId: string, message?: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/repository/push`, {
    method: 'POST',
    body: JSON.stringify({ message: message || '' }),
  })
}

export function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins || 1} 分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} 天前`
  return new Date(iso).toLocaleDateString('zh-CN')
}

export const roleLabels: Record<MemberRole, string> = {
  guest: '访客',
  reporter: '报告者',
  developer: '开发者',
  maintainer: '维护者',
  owner: '所有者',
}
