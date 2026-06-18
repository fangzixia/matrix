import { RobotOutlined } from '@ant-design/icons'
import { Bubble, Sender, Welcome } from '@ant-design/x'
import type { BubbleItemType } from '@ant-design/x'

export type AiMessageRole = 'user' | 'ai' | 'system'

export interface AiMessage {
  key: string | number
  role: AiMessageRole
  content: string
  loading?: boolean
}

export interface MatrixAiChatProps {
  items?: AiMessage[]
  loading?: boolean
  placeholder?: string
  welcomeTitle?: string
  welcomeDescription?: string
  onSubmit: (message: string) => void | Promise<void>
  onCancel?: () => void
  className?: string
}

export default function MatrixAiChat({
  items = [],
  loading = false,
  placeholder = '输入消息…',
  welcomeTitle = 'AI 对话',
  welcomeDescription = '描述你的任务，Matrix 将创建一次运行并执行。',
  onSubmit,
  onCancel,
  className,
}: MatrixAiChatProps) {
  const bubbleItems: BubbleItemType[] = items.map((item) => ({
    key: item.key,
    role: item.role,
    content: item.content,
    loading: item.loading,
  }))

  return (
    <div className={['matrix-ai-chat', className].filter(Boolean).join(' ')}>
      <div className="matrix-ai-chat__body">
        {items.length === 0 ? (
          <Welcome
            icon={<RobotOutlined />}
            title={welcomeTitle}
            description={welcomeDescription}
          />
        ) : (
          <Bubble.List items={bubbleItems} autoScroll />
        )}
      </div>
      <Sender
        loading={loading}
        placeholder={placeholder}
        onSubmit={onSubmit}
        onCancel={onCancel}
      />
    </div>
  )
}
