import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Avatar, Button, Empty, Input, Tabs, Tag } from 'antd'
import { GlobalOutlined, LockOutlined, PlusOutlined } from '@ant-design/icons'
import { useProjectStore } from '@/stores/project'
import * as projectsApi from '@/api/projects'
import { avatarInitials } from '@/utils/avatar'

export default function ProjectsPage() {
  const navigate = useNavigate()
  const projects = useProjectStore((s) => s.projects)
  const fetchProjects = useProjectStore((s) => s.fetchProjects)
  const [scope, setScope] = useState<'yours' | 'explore'>('yours')
  const [filter, setFilter] = useState('')

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return projects
    return projects.filter((p) => p.name.toLowerCase().includes(q))
  }, [filter, projects])

  useEffect(() => {
    fetchProjects(scope)
  }, [scope, fetchProjects])

  return (
    <div className="projects-page">
      <header className="page-header">
        <h1 className="page-title">项目</h1>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/projects/new')}>
          新建项目
        </Button>
      </header>

      <Tabs
        activeKey={scope}
        onChange={(key) => setScope(key as 'yours' | 'explore')}
        items={[
          { key: 'yours', label: '我的项目' },
          { key: 'explore', label: '探索项目' },
        ]}
        style={{ marginBottom: 16 }}
      />

      <div className="projects-page__toolbar" style={{ marginBottom: 16 }}>
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="按名称筛选…"
          allowClear
          style={{ maxWidth: 360 }}
        />
      </div>

      {filtered.length ? (
        <ul className="project-list">
          {filtered.map((p) => {
            const visibility = p.visibility || 'private'
            return (
              <li key={p.id} className="project-list__item">
                <Link to={`/projects/${p.id}`} className="project-list__avatar">
                  <Avatar size={48} shape="square" style={{ backgroundColor: '#fc6d26' }}>
                    {avatarInitials(p.name)}
                  </Avatar>
                </Link>
                <div className="project-list__body">
                  <div className="project-list__title-row">
                    <h2><Link to={`/projects/${p.id}`}>{p.name}</Link></h2>
                    <Tag
                      icon={visibility === 'private' ? <LockOutlined /> : <GlobalOutlined />}
                      title={projectsApi.visibilityTitles[visibility]}
                    >
                      {projectsApi.visibilityLabels[visibility]}
                    </Tag>
                    {p.current_user_role && (
                      <Tag color="blue">{projectsApi.roleLabels[p.current_user_role]}</Tag>
                    )}
                  </div>
                  {p.git_url && <p className="muted project-list__desc">{p.git_url}</p>}
                  <div className="project-list__meta muted">
                    更新于 {projectsApi.formatRelativeTime(p.updated_at)}
                  </div>
                </div>
              </li>
            )
          })}
        </ul>
      ) : (
        <Empty
          description={
            scope === 'explore' ? '暂无可探索的内部或公开项目' : '还没有项目 — 创建第一个项目吧。'
          }
        />
      )}
    </div>
  )
}
