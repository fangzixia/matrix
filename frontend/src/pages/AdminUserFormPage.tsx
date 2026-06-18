import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Alert, Button, Checkbox, Form, Input, Select, Space } from 'antd'
import * as usersApi from '@/api/users'

export default function AdminUserFormPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isNew = !id
  const [error, setError] = useState('')
  const [form] = Form.useForm()

  useEffect(() => {
    if (!isNew && id) {
      usersApi.getUser(id).then((u) => {
        form.setFieldsValue({
          username: u.username,
          email: u.email,
          name: u.name,
          is_admin: u.is_admin,
          state: u.state,
        })
      })
    }
  }, [id, isNew, form])

  async function onFinish(values: {
    username?: string
    email: string
    name: string
    password?: string
    is_admin: boolean
    state?: string
  }) {
    setError('')
    try {
      if (isNew) {
        await usersApi.createUser({
          username: values.username!,
          email: values.email,
          name: values.name,
          password: values.password!,
          is_admin: values.is_admin,
        })
      } else {
        const body: usersApi.UpdateUserInput = {
          email: values.email,
          name: values.name,
          is_admin: values.is_admin,
          state: values.state,
        }
        if (values.password) body.password = values.password
        await usersApi.updateUser(id!, body)
      }
      navigate('/admin/users')
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败')
    }
  }

  return (
    <div>
      <h1 className="page-title">{isNew ? '新建用户' : '编辑用户'}</h1>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} className="panel stack" layout="vertical" style={{ maxWidth: 560 }} onFinish={onFinish} initialValues={{ is_admin: false, state: 'active' }}>
        {isNew ? (
          <Form.Item label="登录名" name="username" rules={[{ required: true }]}>
            <Input placeholder="zhangsan" autoComplete="username" />
          </Form.Item>
        ) : (
          <Form.Item label="登录名">
            <div>@{form.getFieldValue('username')}</div>
          </Form.Item>
        )}
        <Form.Item label="邮箱" name="email" rules={[{ required: true, type: 'email' }]}>
          <Input autoComplete="email" />
        </Form.Item>
        <Form.Item label="显示名称" name="name">
          <Input placeholder="张三" />
        </Form.Item>
        <Form.Item label={isNew ? '密码' : '新密码（可选）'} name="password" rules={isNew ? [{ required: true }] : []}>
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        {!isNew && (
          <Form.Item label="状态" name="state">
            <Select options={[{ value: 'active', label: '正常' }, { value: 'blocked', label: '已封禁' }]} />
          </Form.Item>
        )}
        <Form.Item name="is_admin" valuePropName="checked">
          <Checkbox>
            <strong>管理员</strong>
            <span className="muted" style={{ display: 'block', fontSize: 12 }}>可访问管理区域及全部项目</span>
          </Checkbox>
        </Form.Item>
        <Space>
          <Button onClick={() => navigate(-1)}>取消</Button>
          <Button type="primary" htmlType="submit">保存</Button>
        </Space>
      </Form>
    </div>
  )
}
