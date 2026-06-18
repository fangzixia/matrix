import { useState } from 'react'
import { Alert, Tabs } from 'antd'
import { useAuthStore } from '@/stores/auth'
import SystemAITab from '@/components/admin/system/SystemAITab'
import SystemMCPTab from '@/components/admin/system/SystemMCPTab'
import SystemGitTab from '@/components/admin/system/SystemGitTab'
import SystemWorkerTab from '@/components/admin/system/SystemWorkerTab'
import SystemPipelineTab from '@/components/admin/system/SystemPipelineTab'

export default function AdminSystemSettingsPage() {
  const isRoot = useAuthStore((s) => s.isRoot())
  const [activeTab, setActiveTab] = useState('model')

  if (!isRoot) {
    return <Alert type="error" message="仅 root 用户可访问系统配置。" />
  }

  const items = [
    { key: 'model', label: '模型', children: <SystemAITab /> },
    { key: 'mcp', label: 'MCP 服务', children: <SystemMCPTab /> },
    { key: 'git', label: 'Git 访问', children: <SystemGitTab /> },
    { key: 'worker', label: '并发控制', children: <SystemWorkerTab /> },
    { key: 'pipeline', label: '流水线', children: <SystemPipelineTab /> },
  ]

  return (
    <div>
      <h1>系统配置</h1>
      <p className="muted" style={{ marginBottom: 16 }}>
        按业务域分别管理模型、MCP、Git、任务队列与流水线。各 Tab 独立加载与保存。仅 root 用户可修改。
      </p>
      <Tabs activeKey={activeTab} onChange={setActiveTab} items={items} />
    </div>
  )
}
