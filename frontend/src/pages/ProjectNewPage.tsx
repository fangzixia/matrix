import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Alert, Button, Form, Input, Radio, Space } from 'antd'
import * as projectsApi from '@/api/projects'
import type { ProjectVisibility } from '@/api/projects'

const visibilityOptions: { value: ProjectVisibility; label: string; hint: string }[] = [
  { value: 'private', label: '私有', hint: '仅被明确授权的用户可访问。' },
  { value: 'internal', label: '内部', hint: '所有已登录用户可访问。' },
  { value: 'public', label: '公开', hint: '所有已登录用户可访问。' },
]

export default function ProjectNewPage() {
  const navigate = useNavigate()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function onFinish(values: {
    name: string
    git_url?: string
    git_branch?: string
    visibility: ProjectVisibility
  }) {
    setError('')
    setLoading(true)
    try {
      const p = await projectsApi.createProject({
        name: values.name,
        git_url: values.git_url || undefined,
        git_branch: values.git_branch || undefined,
        visibility: values.visibility,
      })
      navigate(`/projects/${p.id}`)
    } catch (e) {
      setError(e instanceof Error ? e.message : '创建项目失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="new-project">
      <h1 className="page-title">创建新项目</h1>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      <Form
        className="panel stack"
        layout="vertical"
        style={{ maxWidth: 560 }}
        initialValues={{ git_branch: 'main', visibility: 'private' }}
        onFinish={onFinish}
      >
        <Form.Item label="项目名称" name="name" rules={[{ required: true }]}>
          <Input placeholder="my-awesome-project" />
        </Form.Item>
        <Form.Item label="Git 地址（可选）" name="git_url">
          <Input placeholder="https://gitlab.example.com/group/project.git" />
        </Form.Item>
        <Form.Item label="默认分支" name="git_branch">
          <Input placeholder="main" />
        </Form.Item>
        <Form.Item label="可见性" name="visibility">
          <Radio.Group>
            <Space direction="vertical">
              {visibilityOptions.map((opt) => (
                <Radio key={opt.value} value={opt.value}>
                  <strong>{opt.label}</strong>
                  <span className="muted" style={{ marginLeft: 8 }}>{opt.hint}</span>
                </Radio>
              ))}
            </Space>
          </Radio.Group>
        </Form.Item>
        <div className="flex-between">
          <Button onClick={() => navigate('/projects')}>取消</Button>
          <Button type="primary" htmlType="submit" loading={loading}>创建项目</Button>
        </div>
      </Form>
    </div>
  )
}
