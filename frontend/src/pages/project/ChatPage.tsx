import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert } from 'antd'
import { RobotOutlined } from '@ant-design/icons'
import { Bubble, Sender, Welcome } from '@ant-design/x'
import type { BubbleItemType } from '@ant-design/x'
import * as chatApi from '@/api/chat'
import { useTaskStream } from '@/hooks/useTaskStream'

interface ChatItem {
  key: string
  role: 'user' | 'ai'
  content: string
  loading?: boolean
}

function toBubbleItems(items: ChatItem[]): BubbleItemType[] {
  return items.map((item) => ({
    key: item.key,
    role: item.role,
    content: item.content,
    loading: item.loading,
  }))
}

function messagesToItems(messages: chatApi.ChatMessage[]): ChatItem[] {
  return messages.map((m, i) => ({
    key: `hist-${i}`,
    role: m.role === 'user' ? 'user' : 'ai',
    content: m.content,
  }))
}

export default function ChatPage() {
  const { id: projectId = '' } = useParams()
  const { streamTask } = useTaskStream()
  const [sessionId, setSessionId] = useState('')
  const [items, setItems] = useState<ChatItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [booting, setBooting] = useState(true)

  const persistSession = useCallback(async (nextItems: ChatItem[], sid: string) => {
    const messages = nextItems
      .filter((item) => item.content && !item.loading)
      .map((item) => ({
        role: item.role === 'user' ? 'user' as const : 'assistant' as const,
        content: item.content,
      }))
    await chatApi.saveChatSessions(projectId, [{
      id: sid,
      title: messages[0]?.content?.slice(0, 80) || '对话',
      messages,
    }])
  }, [projectId])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setBooting(true)
      try {
        const res = await chatApi.listChatSessions(projectId)
        const sessions = res.sessions ?? []
        if (cancelled) return
        if (sessions.length) {
          const session = sessions[0]
          setSessionId(session.id)
          setItems(messagesToItems(chatApi.parseChatMessages(session.messages)))
        } else {
          const newId = crypto.randomUUID()
          setSessionId(newId)
          setItems([])
        }
      } catch {
        if (!cancelled) {
          setSessionId(crypto.randomUUID())
          setItems([])
        }
      } finally {
        if (!cancelled) setBooting(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [projectId])

  async function send(message: string) {
    const text = message.trim()
    if (!text || !sessionId || loading) return

    setError('')
    setLoading(true)

    const userKey = `user-${Date.now()}`
    const aiKey = `ai-${Date.now()}`
    const withUser: ChatItem[] = [
      ...items,
      { key: userKey, role: 'user', content: text },
      { key: aiKey, role: 'ai', content: '', loading: true },
    ]
    setItems(withUser)

    try {
      const run = await chatApi.sendChatMessage(projectId, sessionId, text)
      const reply = await streamTask(projectId, run.id, (_delta, full) => {
        setItems((prev) => prev.map((item) =>
          item.key === aiKey ? { ...item, content: full, loading: true } : item,
        ))
      })

      const finalItems: ChatItem[] = withUser.map((item) =>
        item.key === aiKey
          ? { ...item, content: reply || '（无回复）', loading: false }
          : item,
      )
      setItems(finalItems)
      await persistSession(finalItems, sessionId)
    } catch (e) {
      setItems((prev) => prev.filter((item) => item.key !== aiKey))
      setError(e instanceof Error ? e.message : '发送失败')
    } finally {
      setLoading(false)
    }
  }

  if (booting) {
    return null
  }

  return (
    <div className="stack">
      <h2>AI 对话</h2>
      {error && <Alert type="error" message={error} />}
      <div className="panel matrix-ai-chat-page">
        <div className="matrix-ai-chat">
          <div className="matrix-ai-chat__body">
            {items.length === 0 ? (
              <Welcome
                icon={<RobotOutlined />}
                title="AI 对话"
                description="与 Matrix AI 对话，获取项目相关帮助。"
              />
            ) : (
              <Bubble.List items={toBubbleItems(items)} autoScroll />
            )}
          </div>
          <Sender
            loading={loading}
            placeholder="输入消息…"
            onSubmit={send}
          />
        </div>
      </div>
    </div>
  )
}
