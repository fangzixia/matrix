/**
 * Chat 会话 API。
 */
import { api } from './client'
import type { Run } from './runs'

export interface ChatMessage {
  role: 'user' | 'assistant' | 'system'
  content: string
}

export interface ChatSession {
  id: string
  title: string
  messages?: string | ChatMessage[]
}

export function parseChatMessages(raw: string | ChatMessage[] | undefined): ChatMessage[] {
  if (!raw) return []
  if (Array.isArray(raw)) return raw
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed.filter((m): m is ChatMessage =>
      !!m && typeof m === 'object' && 'role' in m && 'content' in m)
  } catch {
    return []
  }
}

export function listChatSessions(projectId: string) {
  return api<{ sessions: ChatSession[] }>(`/api/projects/${projectId}/chat/sessions`)
}

export function saveChatSessions(projectId: string, sessions: Array<{ id: string; title: string; messages: ChatMessage[] }>) {
  return api<{ ok: boolean }>(`/api/projects/${projectId}/chat/sessions`, {
    method: 'PUT',
    body: JSON.stringify({
      sessions: sessions.map((s) => ({
        id: s.id,
        title: s.title,
        messages: JSON.stringify(s.messages),
      })),
    }),
  })
}

export function sendChatMessage(projectId: string, sessionId: string, message: string) {
  return api<Run>(`/api/projects/${projectId}/chat/sessions/${sessionId}/run`, {
    method: 'POST',
    body: JSON.stringify({ message }),
  })
}
