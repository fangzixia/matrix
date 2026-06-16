import { api } from './client'

export interface ModelProfile {
  id: string
  name: string
  base_url: string
  api_key?: string
  api_key_set?: boolean
  model: string
  max_tokens: number
  enabled: boolean
  default: boolean
}

export interface SystemContextSettings {
  auto_compact_threshold: number
  keep_recent_messages: number
}

export interface SystemSecuritySettings {
  allow_shell: boolean
  allow_command_mcp: boolean
  shell_timeout: string
}

export interface SystemMCPServer {
  command?: string
  args?: string[]
  url?: string
  disabled?: boolean
  headers?: Record<string, string>
  env?: Record<string, string>
}

export interface GitAccess {
  id: string
  name: string
  host: string
  ssh_key_path: string
}

export interface SystemGitSettings {
  clone_timeout: string
  ssh_key_path?: string
  accesses: GitAccess[]
  platform?: string
  platform_label?: string
  default_ssh_key_path?: string
}

export interface SystemWorkerSettings {
  enabled: boolean
  poll_interval: string
  max_attempts: number
  concurrency: number
}

export interface SystemPipelineSettings {
  default_stages: string[]
  pull_before_stage: boolean
}

export interface SystemSettings {
  models: ModelProfile[]
  context: SystemContextSettings
  security: SystemSecuritySettings
  mcp_servers: Record<string, SystemMCPServer>
  git: SystemGitSettings
  worker: SystemWorkerSettings
  pipeline: SystemPipelineSettings
}

export function getSystemSettings() {
  return api<SystemSettings>('/api/admin/system/settings')
}

export function saveSystemSettings(body: SystemSettings) {
  return api<SystemSettings>('/api/admin/system/settings', {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export function saveMcpSettings(mcp_servers: Record<string, SystemMCPServer>) {
  return api<SystemSettings>('/api/admin/system/settings/mcp', {
    method: 'PUT',
    body: JSON.stringify({ mcp_servers }),
  })
}

export function testGitAccess(git_url: string) {
  return api<{ ok: boolean; message: string }>('/api/admin/system/git/test', {
    method: 'POST',
    body: JSON.stringify({ git_url }),
  })
}
