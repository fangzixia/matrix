/**
 * Run 与用户通知 SSE 流订阅。
 */
import type { Notification } from './notifications'

/** SSE 消息回调 */
export type StreamHandler = (data: unknown) => void

/** 通知 SSE 推送载荷 */
export interface NotificationStreamPayload {
  type?: string
  notification: Notification
}

/**
 * subscribeRunStream 订阅 Run 执行流，返回取消订阅函数。
 */
export function subscribeRunStream(projectId: string, runId: string, onMessage: StreamHandler): () => void {
  const url = `/api/projects/${projectId}/runs/${runId}/stream`
  const es = new EventSource(url, { withCredentials: true })

  es.addEventListener('agent:stream', (ev) => {
    try {
      onMessage(JSON.parse((ev as MessageEvent).data))
    } catch {
      onMessage((ev as MessageEvent).data)
    }
  })

  es.onerror = () => {
    es.close()
  }

  return () => es.close()
}

/**
 * subscribeNotificationStream 订阅用户通知 SSE，返回取消订阅函数。
 */
export function subscribeNotificationStream(
  onMessage: (payload: NotificationStreamPayload) => void,
): () => void {
  const es = new EventSource('/api/notifications/stream', { withCredentials: true })

  es.addEventListener('notification', (ev) => {
    try {
      onMessage(JSON.parse((ev as MessageEvent).data) as NotificationStreamPayload)
    } catch {
      /* ignore malformed payloads */
    }
  })

  es.onerror = () => {
    es.close()
  }

  return () => es.close()
}
