import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Alert, Button, Input, Select, Table, Tag } from 'antd'
import * as runsApi from '@/api/runs'
import * as projectsApi from '@/api/projects'
import { useRunStore } from '@/stores/run'
import { harnessKindHints, runKindLabels, runStatusLabels } from '@/locales/zh-CN'

const taskKinds = [
  'task', 'chat', 'spec', 'implement', 'verify', 'build',
].map((value) => ({ value, label: runKindLabels[value] }))

function statusColor(status: string) {
  if (status === 'succeeded') return 'success'
  if (status === 'failed' || status === 'cancelled') return 'error'
  if (status === 'running') return 'processing'
  return 'default'
}

export default function RunsPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const runs = useRunStore((s) => s.runs)
  const fetchRuns = useRunStore((s) => s.fetchRuns)
  const [message, setMessage] = useState('')
  const [filePath, setFilePath] = useState('')
  const [kind, setKind] = useState('task')
  const [pipelineMessage, setPipelineMessage] = useState('')
  const [requirements, setRequirements] = useState<projectsApi.RequirementItem[]>([])
  const [starting, setStarting] = useState(false)

  useEffect(() => { fetchRuns(id) }, [id, fetchRuns])

  useEffect(() => {
    projectsApi.listRequirements(id).then((res) => {
      setRequirements(res.requirements ?? [])
    }).catch(() => setRequirements([]))
  }, [id])

  useEffect(() => {
    const qKind = searchParams.get('kind')
    const qFile = searchParams.get('file')
    const qMessage = searchParams.get('message')
    if (qKind) setKind(qKind)
    if (qFile) setFilePath(qFile)
    if (qMessage) setMessage(qMessage)
  }, [searchParams])

  const kindHint = harnessKindHints[kind]

  const requirementOptions = useMemo(
    () => requirements.map((r) => ({ value: r.path, label: r.title || r.path })),
    [requirements],
  )

  async function startRun() {
    setStarting(true)
    try {
      const run = await runsApi.startRun(id, message || '新任务', kind, filePath)
      setMessage('')
      navigate(`/projects/${id}/runs/${run.id}`)
    } finally {
      setStarting(false)
    }
  }

  async function startPipeline() {
    setStarting(true)
    try {
      const run = await runsApi.startPipeline(id, pipelineMessage || '流水线运行')
      setPipelineMessage('')
      navigate(`/projects/${id}/runs/${run.id}`)
    } finally {
      setStarting(false)
    }
  }

  return (
    <div>
      <div className="flex-between"><h2>运行</h2></div>
      <div className="panel stack" style={{ marginBottom: 16 }} data-testid="single-run-panel">
        <h3>单次运行</h3>
        <Select value={kind} onChange={setKind} options={taskKinds} />
        {kindHint && <Alert type="info" showIcon message={kindHint} />}
        <Input value={message} onChange={(e) => setMessage(e.target.value)} placeholder="任务描述" />
        {requirementOptions.length > 0 ? (
          <Select
            allowClear
            showSearch
            placeholder="选择需求文件（可选）"
            value={filePath || undefined}
            onChange={(v) => setFilePath(v ?? '')}
            options={requirementOptions}
          />
        ) : (
          <Input value={filePath} onChange={(e) => setFilePath(e.target.value)} placeholder="规格文件路径（可选）" />
        )}
        <Button type="primary" loading={starting} onClick={startRun}>启动运行</Button>
      </div>
      <div className="panel stack" style={{ marginBottom: 16 }} data-testid="pipeline-panel">
        <h3>流水线（规格 → 实现 → 验证 → 构建）</h3>
        <Input value={pipelineMessage} onChange={(e) => setPipelineMessage(e.target.value)} placeholder="流水线描述" />
        <Button type="primary" loading={starting} onClick={startPipeline}>启动流水线</Button>
        <p className="muted">异步执行，由 Worker 消费队列。</p>
      </div>
      <Table dataSource={runs} rowKey="id" pagination={{ pageSize: 20 }}>
        <Table.Column title="标题" render={(_, row: runsApi.Run) => (
          <Link to={`/projects/${id}/runs/${row.id}`}>{row.title || row.id}</Link>
        )} />
        <Table.Column title="类型" render={(_, row: runsApi.Run) => runKindLabels[row.kind] || row.kind} />
        <Table.Column title="状态" render={(_, row: runsApi.Run) => (
          <Tag color={statusColor(row.status)}>{runStatusLabels[row.status] || row.status}</Tag>
        )} />
        <Table.Column title="创建时间" render={(_, row: runsApi.Run) => (
          new Date(row.created_at).toLocaleString('zh-CN')
        )} />
      </Table>
    </div>
  )
}
