import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import { Alert, Breadcrumb, Button, Input, Select, Table, Tag } from 'antd'
import * as runsApi from '@/api/runs'
import * as projectsApi from '@/api/projects'
import { useRunStore } from '@/stores/run'
import { harnessKindHints, runKindLabels, runStatusLabels, stageTitles } from '@/locales/zh-CN'
import { isStageKind, stageKindFromPath } from '@/utils/stage'

function statusColor(status: string) {
  if (status === 'succeeded') return 'success'
  if (status === 'failed' || status === 'cancelled') return 'error'
  if (status === 'running') return 'processing'
  return 'default'
}

export default function StagePage() {
  const { id = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const pathKind = stageKindFromPath(location.pathname)
  const kind = pathKind && isStageKind(pathKind) ? pathKind : 'plan'
  const runs = useRunStore((s) => s.runs)
  const fetchRuns = useRunStore((s) => s.fetchRuns)
  const [message, setMessage] = useState('')
  const [filePath, setFilePath] = useState('')
  const [plans, setPlans] = useState<projectsApi.PlanItem[]>([])
  const [starting, setStarting] = useState(false)

  useEffect(() => { fetchRuns(id, kind) }, [id, kind, fetchRuns])

  useEffect(() => {
    projectsApi.listPlans(id).then((res) => {
      setPlans(res.plans ?? [])
    }).catch(() => setPlans([]))
  }, [id])

  const kindHint = harnessKindHints[kind]
  const title = stageTitles[kind] || kind

  const planOptions = useMemo(
    () => plans.map((r) => ({ value: r.path, label: r.title || r.path })),
    [plans],
  )

  const stageTasks = useMemo(
    () => runs.filter((r) => r.kind === kind),
    [runs, kind],
  )

  async function startTask() {
    setStarting(true)
    try {
      const run = await runsApi.startRun(id, message || `${runKindLabels[kind]}任务`, kind, filePath)
      setMessage('')
      navigate(`/projects/${id}/${kind}/${run.id}`)
    } finally {
      setStarting(false)
    }
  }

  const needsPlanFile = kind === 'plan' || kind === 'implement' || kind === 'verify'

  return (
    <div>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to={`/projects/${id}`}>概览</Link> },
          { title },
        ]}
      />
      <h2>{title}</h2>
      <div className="panel stack" style={{ marginBottom: 16 }} data-testid="stage-start-panel">
        {kindHint && <Alert type="info" showIcon message={kindHint} />}
        <Input value={message} onChange={(e) => setMessage(e.target.value)} placeholder="任务描述" />
        {needsPlanFile && (
          planOptions.length > 0 ? (
            <Select
              allowClear
              showSearch
              placeholder="选择计划文件（可选）"
              value={filePath || undefined}
              onChange={(v) => setFilePath(v ?? '')}
              options={planOptions}
            />
          ) : (
            <Input value={filePath} onChange={(e) => setFilePath(e.target.value)} placeholder="计划文件路径（可选）" />
          )
        )}
        <Button type="primary" loading={starting} onClick={startTask}>启动</Button>
      </div>
      <Table dataSource={stageTasks} rowKey="id" pagination={{ pageSize: 20 }}>
        <Table.Column title="标题" render={(_, row: runsApi.Run) => (
          <Link to={`/projects/${id}/${kind}/${row.id}`}>{row.title || row.id}</Link>
        )} />
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
