import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Button,
  Descriptions,
  Drawer,
  Empty,
  Input,
  List,
  Space,
  Spin,
  Steps,
  Tag,
  Typography,
} from 'antd'
import { GlobalOutlined, LockOutlined } from '@ant-design/icons'
import { useProjectStore } from '@/stores/project'
import * as projectsApi from '@/api/projects'
import * as runsApi from '@/api/runs'
import { runStatusLabels } from '@/locales/zh-CN'

type DocItem = projectsApi.RequirementItem | projectsApi.EvaluationItem

const workflowStages = [
  { key: 'spec', title: '编写需求', description: '产出 SPEC 需求文档' },
  { key: 'implement', title: '编码实现', description: '按需求完成代码' },
  { key: 'verify', title: '验证评测', description: '对照 AC 验收' },
  { key: 'build', title: '构建', description: '构建与验证' },
] as const

export default function OverviewPage() {
  const { id: projectId = '' } = useParams()
  const navigate = useNavigate()
  const current = useProjectStore((s) => s.current)
  const [requirements, setRequirements] = useState<projectsApi.RequirementItem[]>([])
  const [evaluations, setEvaluations] = useState<projectsApi.EvaluationItem[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedPath, setSelectedPath] = useState('')
  const [preview, setPreview] = useState<{ title: string; content: string } | null>(null)
  const [pipelineMessage, setPipelineMessage] = useState('')
  const [pipelineStarting, setPipelineStarting] = useState(false)
  const [lastPipeline, setLastPipeline] = useState<runsApi.Run | null>(null)

  useEffect(() => {
    if (!projectId) return
    setLoading(true)
    Promise.all([
      projectsApi.listRequirements(projectId),
      projectsApi.listEvaluations(projectId),
      runsApi.listRuns(projectId),
    ])
      .then(([req, ev, runs]) => {
        const reqs = req.requirements ?? []
        setRequirements(reqs)
        setEvaluations(ev.evaluations ?? [])
        const pipeline = (runs.runs ?? []).find((r) => r.kind === 'pipeline') ?? null
        setLastPipeline(pipeline)
        if (reqs[0]?.path) {
          setSelectedPath((prev) => prev || reqs[0].path)
        }
      })
      .catch(() => {
        setRequirements([])
        setEvaluations([])
        setLastPipeline(null)
      })
      .finally(() => setLoading(false))
  }, [projectId])

  const visibility = current?.visibility || 'private'

  const workflowStep = useMemo(() => {
    if (!lastPipeline) return 0
    if (lastPipeline.status === 'succeeded') return 4
    if (lastPipeline.status === 'failed' || lastPipeline.status === 'cancelled') return 3
    if (lastPipeline.status === 'running' || lastPipeline.status === 'queued') return 1
    return 0
  }, [lastPipeline])

  function openDoc(item: DocItem) {
    setPreview({
      title: item.title || item.path,
      content: item.content || '（无内容）',
    })
  }

  function goRun(kind: string, message?: string) {
    const params = new URLSearchParams({ kind })
    if (selectedPath) params.set('file', selectedPath)
    if (message) params.set('message', message)
    navigate(`/projects/${projectId}/runs?${params.toString()}`)
  }

  async function startPipeline() {
    setPipelineStarting(true)
    try {
      const run = await runsApi.startPipeline(
        projectId,
        pipelineMessage || '一键流水线运行',
      )
      navigate(`/projects/${projectId}/runs/${run.id}`)
    } finally {
      setPipelineStarting(false)
    }
  }

  if (loading && !current) {
    return <Spin style={{ display: 'block', margin: '48px auto' }} />
  }

  return (
    <div className="overview">
      {current && (
        <header style={{ marginBottom: 20 }}>
          <h1 className="page-title">{current.name}</h1>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 8 }}>
            <Tag
              icon={visibility === 'private' ? <LockOutlined /> : <GlobalOutlined />}
              title={projectsApi.visibilityTitles[visibility]}
            >
              {projectsApi.visibilityLabels[visibility]}
            </Tag>
            {current.current_user_role && (
              <Tag color="blue">{projectsApi.roleLabels[current.current_user_role]}</Tag>
            )}
          </div>
        </header>
      )}
      <div className="stack">
        <section className="panel" data-testid="harness-workflow">
          <h2 className="panel__title">需求 → 编码 → 测试</h2>
          <p className="muted" style={{ marginBottom: 16 }}>
            按 Harness 流水线驱动：编写需求、编码实现、验证评测与构建。可在下方选择需求文件后启动各阶段。
          </p>
          <Steps
            current={workflowStep}
            size="small"
            items={workflowStages.map((s) => ({
              title: s.title,
              description: s.description,
            }))}
            style={{ marginBottom: 20 }}
          />
          {lastPipeline && (
            <p className="muted" style={{ marginBottom: 12 }}>
              最近流水线：
              {' '}
              <Tag color={lastPipeline.status === 'succeeded' ? 'success' : lastPipeline.status === 'failed' ? 'error' : 'processing'}>
                {runStatusLabels[lastPipeline.status] || lastPipeline.status}
              </Tag>
              {lastPipeline.title}
            </p>
          )}
          <Space wrap style={{ marginBottom: 16 }}>
            <Button onClick={() => goRun('spec', '编写需求文档')}>编写需求</Button>
            <Button onClick={() => goRun('implement', '按需求文档实现')}>编码实现</Button>
            <Button onClick={() => goRun('verify', '验证当前实现')}>验证评测</Button>
            <Button onClick={() => goRun('build', '执行构建')}>执行构建</Button>
          </Space>
          <div className="stack" style={{ maxWidth: 480 }}>
            <Input
              value={pipelineMessage}
              onChange={(e) => setPipelineMessage(e.target.value)}
              placeholder="流水线描述（可选）"
            />
            <Button type="primary" loading={pipelineStarting} onClick={startPipeline}>
              一键流水线
            </Button>
          </div>
        </section>

        <section className="panel">
          <h2 className="panel__title">项目信息</h2>
          <Descriptions column={1} size="small" colon>
            <Descriptions.Item label="Git 仓库">{current?.git_url || '—'}</Descriptions.Item>
            <Descriptions.Item label="默认分支">
              <code>{current?.git_branch || 'main'}</code>
            </Descriptions.Item>
          </Descriptions>
        </section>

        <section className="panel">
          <h2 className="panel__title">需求文档 (.matrix/SPEC-*.md)</h2>
          {requirements.length ? (
            <List
              dataSource={requirements}
              renderItem={(item) => (
                <List.Item
                  className={selectedPath === item.path ? 'doc-item doc-item--selected' : 'doc-item'}
                  onClick={() => {
                    setSelectedPath(item.path)
                    openDoc(item)
                  }}
                  style={{ cursor: 'pointer' }}
                >
                  <List.Item.Meta
                    title={item.title || item.path}
                    description={<Typography.Text type="secondary" style={{ fontSize: 12 }}>{item.path}</Typography.Text>}
                  />
                </List.Item>
              )}
            />
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无需求文档">
              <Button type="primary" onClick={() => goRun('spec', '编写首个需求文档')}>
                编写需求
              </Button>
            </Empty>
          )}
        </section>

        <section className="panel">
          <h2 className="panel__title">评测报告</h2>
          {evaluations.length ? (
            <List
              dataSource={evaluations}
              renderItem={(item) => (
                <List.Item className="doc-item" onClick={() => openDoc(item)} style={{ cursor: 'pointer' }}>
                  <List.Item.Meta
                    title={item.title || item.path}
                    description={<Typography.Text type="secondary" style={{ fontSize: 12 }}>{item.path}</Typography.Text>}
                  />
                </List.Item>
              )}
            />
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无评测报告" />
          )}
        </section>
      </div>

      <Drawer
        title={preview?.title}
        open={!!preview}
        onClose={() => setPreview(null)}
        width={560}
        data-testid="doc-preview-drawer"
      >
        <Input.TextArea value={preview?.content} readOnly autoSize={{ minRows: 12, maxRows: 32 }} />
      </Drawer>
    </div>
  )
}
