import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert, Button, Form, Input, InputNumber, Tabs } from 'antd'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/hooks/useProjectPermissions'
import { useSettingsTabNavigate } from '@/hooks/useSettingsTabNavigate'
import { getIntegrations, saveIntegrations, type IntegrationSettings } from '@/api/projects'
import { settingsTabs } from '@/locales/zh-CN'

export default function SettingsIntegrationsPage() {
  const { id = '' } = useParams()
  const projectStore = useProjectStore()
  const perms = useProjectPermissions(projectStore.current)
  const onSettingsTabChange = useSettingsTabNavigate(id)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [mcpJson, setMcpJson] = useState('{}')
  const [form] = Form.useForm()

  async function load() {
    const s = await getIntegrations(id)
    if (s.model) {
      form.setFieldsValue({ ...s.model })
    }
    setMcpJson(JSON.stringify(s.mcp_servers || {}, null, 2))
  }

  useEffect(() => { load() }, [id])

  async function onFinish(values: NonNullable<IntegrationSettings['model']>) {
    setError('')
    setMessage('')
    try {
      let mcp_servers = {}
      if (mcpJson.trim()) mcp_servers = JSON.parse(mcpJson)
      await saveIntegrations(id, { model: values, mcp_servers })
      setMessage('已保存')
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败')
    }
  }

  if (!perms.canManageSettings) {
    return <Alert type="error" message="您没有权限访问集成设置。" />
  }

  return (
    <div>
      <Tabs
        activeKey="integrations"
        onChange={onSettingsTabChange}
        items={settingsTabs(id).map((tab) => ({ key: tab.key, label: tab.label }))}
        style={{ marginBottom: 16 }}
      />
      <h2>集成</h2>
      <p className="muted">项目级模型与 MCP 覆盖（留空则使用系统 YAML 默认）。</p>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      {message && <Alert type="success" message={message} style={{ marginBottom: 16 }} />}
      <Form form={form} className="panel stack" layout="vertical" initialValues={{ max_tokens: 8192 }} onFinish={onFinish}>
        <Form.Item label="模型 API 地址" name="base_url">
          <Input placeholder="https://api.deepseek.com" />
        </Form.Item>
        <Form.Item label="API Key" name="api_key">
          <Input.Password />
        </Form.Item>
        <Form.Item label="模型名称" name="model">
          <Input placeholder="deepseek-chat" />
        </Form.Item>
        <Form.Item label="最大 Token" name="max_tokens">
          <InputNumber min={1} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item label="MCP 服务（JSON）">
          <Input.TextArea rows={8} value={mcpJson} onChange={(e) => setMcpJson(e.target.value)} />
        </Form.Item>
        <Button type="primary" htmlType="submit">保存</Button>
      </Form>
    </div>
  )
}
