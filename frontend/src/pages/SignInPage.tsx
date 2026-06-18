import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Alert, Button, Checkbox, Form, Input } from 'antd'
import { useAuthStore } from '@/stores/auth'
import { resolvePostLoginRedirect } from '@/router/guards'

export default function SignInPage() {
  const auth = useAuthStore()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function onFinish(values: { login: string; password: string }) {
    setError('')
    setLoading(true)
    try {
      await auth.login(values.login, values.password)
      const redirect = resolvePostLoginRedirect(searchParams.get('redirect'))
      navigate(redirect)
    } catch (e) {
      setError(e instanceof Error ? e.message : '登录名或密码错误。')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="sign-in">
      <h2 className="sign-in__title">登录</h2>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      <Form layout="vertical" onFinish={onFinish} className="sign-in__form">
        <Form.Item label="登录名或邮箱" name="login" rules={[{ required: true }]}>
          <Input autoComplete="username" />
        </Form.Item>
        <Form.Item label="密码" name="password" rules={[{ required: true }]}>
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Form.Item name="remember" valuePropName="checked">
          <Checkbox>记住我</Checkbox>
        </Form.Item>
        <Button type="primary" htmlType="submit" loading={loading} block className="sign-in__submit">
          登录
        </Button>
      </Form>
    </div>
  )
}
