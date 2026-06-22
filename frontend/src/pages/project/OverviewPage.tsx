import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import {
  Descriptions,
  Drawer,
  Empty,
  Input,
  List,
  Spin,
  Tag,
  Typography,
} from 'antd'
import { GlobalOutlined, LockOutlined } from '@ant-design/icons'
import { useProjectStore } from '@/stores/project'
import * as projectsApi from '@/api/projects'

type DocItem = projectsApi.PlanItem | projectsApi.EvaluationItem

export default function OverviewPage() {
  const { id: projectId = '' } = useParams()
  const current = useProjectStore((s) => s.current)
  const [plans, setPlans] = useState<projectsApi.PlanItem[]>([])
  const [evaluations, setEvaluations] = useState<projectsApi.EvaluationItem[]>([])
  const [loading, setLoading] = useState(true)
  const [preview, setPreview] = useState<{ title: string; content: string } | null>(null)

  useEffect(() => {
    if (!projectId) return
    setLoading(true)
    Promise.all([
      projectsApi.listPlans(projectId),
      projectsApi.listEvaluations(projectId),
    ])
      .then(([planRes, ev]) => {
        setPlans(planRes.plans ?? [])
        setEvaluations(ev.evaluations ?? [])
      })
      .catch(() => {
        setPlans([])
        setEvaluations([])
      })
      .finally(() => setLoading(false))
  }, [projectId])

  const visibility = current?.visibility || 'private'

  function openDoc(item: DocItem) {
    setPreview({
      title: item.title || item.path,
      content: item.content || '（无内容）',
    })
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
          <h2 className="panel__title">计划文档 (.matrix/PLAN-*.md)</h2>
          {plans.length ? (
            <List
              dataSource={plans}
              renderItem={(item) => (
                <List.Item
                  className="doc-item"
                  onClick={() => openDoc(item)}
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
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无计划文档">
              <Link to={`/projects/${projectId}/plan`}>前往编写计划</Link>
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
