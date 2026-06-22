import { useEffect, useState } from 'react'
import { Alert, Button, Checkbox, Form, Input, Spin } from 'antd'
import { getPipelineSettings, savePipelineSettings, type SystemPipelineSettings } from '@/api/system'

export default function SystemPipelineTab() {
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [stagesText, setStagesText] = useState('')
  const [form, setForm] = useState<SystemPipelineSettings>({
    default_stages: ['plan', 'implement', 'verify', 'build'],
    pull_before_stage: true,
  })

  useEffect(() => {
    getPipelineSettings().then((s) => {
      setForm(s)
      setStagesText((s.default_stages || []).join(', '))
      setLoaded(true)
    })
  }, [])

  async function save() {
    setError('')
    setMessage('')
    setSaving(true)
    try {
      const stages = stagesText.split(',').map((s) => s.trim()).filter(Boolean)
      const saved = await savePipelineSettings({ ...form, default_stages: stages })
      setForm(saved)
      setStagesText((saved.default_stages || []).join(', '))
      setMessage('流水线配置已保存')
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (!loaded) return <Spin style={{ display: 'block', margin: '24px auto' }} />

  return (
    <section className="stack">
      {error && <Alert type="error" message={error} />}
      {message && <Alert type="success" message={message} />}
      <h2>流水线</h2>
      <Form layout="vertical">
        <Form.Item label="默认阶段（逗号分隔）">
          <Input value={stagesText} onChange={(e) => setStagesText(e.target.value)} placeholder="plan, implement, verify, build" />
        </Form.Item>
        <Form.Item label="阶段前拉取代码">
          <Checkbox checked={form.pull_before_stage} onChange={(e) => setForm({ ...form, pull_before_stage: e.target.checked })}>启用</Checkbox>
        </Form.Item>
      </Form>
      <Button type="primary" loading={saving} onClick={save}>保存流水线配置</Button>
    </section>
  )
}
