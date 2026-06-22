import { useCallback, useRef } from 'react'
import { subscribeRunStream } from '@/api/stream'
import * as runsApi from '@/api/runs'

interface StreamMessage {
  type?: string
  event?: {
    type?: string
    delta?: { type?: string; text?: string }
  }
  message?: {
    content?: Array<{ type?: string; text?: string }>
  }
  output?: string
}

/** 从 SSE 载荷中提取可追加的文本片段。 */
export function extractStreamText(data: unknown): string | null {
  if (!data || typeof data !== 'object') return null
  const msg = data as StreamMessage

  if (msg.type === 'stream_event' && msg.event?.type === 'content_block_delta') {
    const delta = msg.event.delta
    if (delta?.type === 'text_delta' && delta.text) return delta.text
  }

  if (msg.type === 'assistant' && msg.message?.content) {
    const text = msg.message.content
      .filter((b) => b.type === 'text' && b.text)
      .map((b) => b.text)
      .join('')
    if (text) return text
  }

  if (msg.type === 'result' && msg.output) return msg.output

  return null
}

function extractAssistantFromEvents(events: runsApi.RunEvent[]): string {
  for (let i = events.length - 1; i >= 0; i--) {
    const payload = events[i].payload
    if (!payload) continue
    try {
      const parsed = JSON.parse(payload) as StreamMessage
      if (parsed.type === 'assistant' && parsed.message?.content) {
        const text = parsed.message.content
          .filter((b) => b.type === 'text' && b.text)
          .map((b) => b.text)
          .join('')
        if (text) return text
      }
      if (parsed.type === 'result' && parsed.output) return parsed.output
    } catch {
      /* ignore */
    }
  }
  return ''
}

/**
 * 订阅任务 SSE 并在完成时解析最终回复文本。
 */
export function useTaskStream() {
  const unsubscribeRef = useRef<(() => void) | null>(null)

  const stop = useCallback(() => {
    unsubscribeRef.current?.()
    unsubscribeRef.current = null
  }, [])

  const streamTask = useCallback(async (
    projectId: string,
    taskId: string,
    onDelta: (text: string, full: string) => void,
  ): Promise<string> => {
    stop()
    let full = ''

    unsubscribeRef.current = subscribeRunStream(projectId, taskId, (data) => {
      const chunk = extractStreamText(data)
      if (chunk) {
        if (data && typeof data === 'object' && (data as StreamMessage).type === 'assistant') {
          full = chunk
        } else {
          full += chunk
        }
        onDelta(chunk, full)
      }
    })

    for (;;) {
      const run = await runsApi.getRun(projectId, taskId)
      if (!['running', 'queued', 'pending'].includes(run.status)) {
        break
      }
      await new Promise((r) => setTimeout(r, 2000))
    }

    stop()

    if (full.trim()) return full.trim()

    const eventsRes = await runsApi.listRunEvents(projectId, taskId)
    const fromEvents = extractAssistantFromEvents(eventsRes.events)
    if (fromEvents) return fromEvents.trim()

    try {
      const audit = await runsApi.getRunAudit(projectId, taskId)
      if (audit.content?.trim()) return audit.content.trim()
    } catch {
      /* ignore */
    }

    return full.trim()
  }, [stop])

  return { streamTask, stop }
}
