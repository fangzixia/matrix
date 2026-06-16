import { api } from './client'

export interface Repository {
  id: string
  project_id: string
  name: string
  git_url: string
  git_branch: string
  is_default: boolean
  created_at: string
  updated_at: string
}

export function listRepositories(projectId: string) {
  return api<{ repositories: Repository[] }>(`/api/projects/${projectId}/repositories`)
}

export function createRepository(projectId: string, body: {
  name: string
  git_url?: string
  git_branch?: string
  is_default?: boolean
}) {
  return api<Repository>(`/api/projects/${projectId}/repositories`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export function updateRepository(projectId: string, repoId: string, body: {
  name?: string
  git_url?: string
  git_branch?: string
  is_default?: boolean
}) {
  return api<Repository>(`/api/projects/${projectId}/repositories/${repoId}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function deleteRepository(projectId: string, repoId: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/repositories/${repoId}`, { method: 'DELETE' })
}

export function pullRepo(projectId: string, repoId: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/repositories/${repoId}/pull`, { method: 'POST' })
}

export function pushRepo(projectId: string, repoId: string, message?: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/repositories/${repoId}/push`, {
    method: 'POST',
    body: JSON.stringify({ message: message || '' }),
  })
}
