/**
 * Run、流水线、Chat 会话 API。
 */
import { api } from './client'

/** AI 任务/Chat 运行记录 */
export interface Run {
  id: string
  project_id: string
  repository_id?: string
  kind: string
  status: string
  title?: string
  audit_path?: string
  error_message?: string
  started_at?: string
  finished_at?: string
  created_at: string
}

/** Run 内流水线步骤 */
export interface RunStep {
  id: string
  run_id: string
  kind: string
  sequence: number
  status: string
  output_summary?: string
  started_at?: string
  finished_at?: string
}

/** Run 事件快照 */
export interface RunEvent {
  id: string
  run_id: string
  step_id?: string
  event_type: string
  payload?: string
  created_at: string
}

/** Chat 会话元数据 */
export interface ChatSession {
  id: string
  title: string
  messages?: unknown[]
}

export function listRuns(projectId: string) {
  return api<{ runs: Run[] }>(`/api/projects/${projectId}/runs`)
}

export function startRun(projectId: string, message: string, kind = 'task', filePath = '', sync = false) {
  const q = sync ? '?sync=1' : ''
  return api<Run>(`/api/projects/${projectId}/runs${q}`, {
    method: 'POST',
    body: JSON.stringify({ message, kind, file_path: filePath }),
  })
}

export function startPipeline(projectId: string, message: string, stages?: string[], repositoryId?: string) {
  return api<Run>(`/api/projects/${projectId}/pipelines`, {
    method: 'POST',
    body: JSON.stringify({ message, stages, repository_id: repositoryId || undefined }),
  })
}

export function getRun(projectId: string, runId: string) {
  return api<Run>(`/api/projects/${projectId}/runs/${runId}`)
}

export function listRunSteps(projectId: string, runId: string) {
  return api<{ steps: RunStep[] }>(`/api/projects/${projectId}/runs/${runId}/steps`)
}

export function listRunEvents(projectId: string, runId: string, afterId?: string) {
  const q = afterId ? `?after_id=${afterId}` : ''
  return api<{ events: RunEvent[] }>(`/api/projects/${projectId}/runs/${runId}/events${q}`)
}

export function getRunAudit(projectId: string, runId: string) {
  return api<{ content: string }>(`/api/projects/${projectId}/runs/${runId}/audit`)
}

export function cancelRun(projectId: string, runId: string) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/runs/${runId}/cancel`, { method: 'POST' })
}

export function listChatSessions(projectId: string) {
  return api<{ sessions: ChatSession[] }>(`/api/projects/${projectId}/chat/sessions`)
}

export function saveChatSessions(projectId: string, sessions: ChatSession[]) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/chat/sessions`, {
    method: 'PUT',
    body: JSON.stringify({ sessions }),
  })
}

export function runChat(projectId: string, sessionId: string, message: string) {
  return api<Run>(`/api/projects/${projectId}/chat/sessions/${sessionId}/run`, {
    method: 'POST',
    body: JSON.stringify({ message }),
  })
}
