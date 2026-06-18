import { useState } from 'react'
import { Alert, Button, Form, Input } from 'antd'
import { useAuthStore } from '@/stores/auth'
import * as authApi from '@/api/auth'

export default function ProfilePage() {
  const auth = useAuthStore()
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  async function onFinish(values: { name: string; email: string; password?: string }) {
    setError('')
    setMessage('')
    try {
      const body: { name?: string; email?: string; password?: string } = {
        name: values.name,
        email: values.email,
      }
      if (values.password) body.password = values.password
      const user = await authApi.updateProfile(body)
      auth.setUser(user)
      setMessage('已保存')
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败')
    }
  }

  return (
    <div>
      <h1 className="page-title">个人资料</h1>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      {message && <Alert type="success" message={message} style={{ marginBottom: 16 }} />}
      <Form
        className="panel stack"
        layout="vertical"
        style={{ maxWidth: 480 }}
        initialValues={{
          name: auth.user?.name || '',
          email: auth.user?.email || '',
        }}
        onFinish={onFinish}
      >
        <Form.Item label="登录名">
          <div>@{auth.user?.username}</div>
          <span className="muted" style={{ fontSize: 12 }}>创建后不可修改，用于登录与 @ 提及。</span>
        </Form.Item>
        <Form.Item label="显示名称" name="name">
          <Input placeholder="张三" />
        </Form.Item>
        <Form.Item label="邮箱" name="email" rules={[{ type: 'email' }]}>
          <Input autoComplete="email" />
        </Form.Item>
        <Form.Item label="新密码" name="password">
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Button type="primary" htmlType="submit">保存更改</Button>
      </Form>
    </div>
  )
}
