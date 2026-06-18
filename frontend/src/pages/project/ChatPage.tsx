import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Alert } from 'antd'
import { MatrixAiChat } from '@/components/ai'
import * as runsApi from '@/api/runs'

export default function ChatPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function send(message: string) {
    if (!message.trim()) return
    setError('')
    setLoading(true)
    try {
      const run = await runsApi.runChat(id, 'default', message)
      navigate(`/projects/${id}/runs/${run.id}`)
    } catch (e) {
      setError(e instanceof Error ? e.message : '发送失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="stack">
      <h2>AI 对话</h2>
      {error && <Alert type="error" message={error} />}
      <div className="panel matrix-ai-chat-page">
        <MatrixAiChat loading={loading} onSubmit={send} />
      </div>
    </div>
  )
}
