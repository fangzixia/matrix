import { useEffect, useMemo, useState, type ComponentType, type CSSProperties } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'
import { Avatar, Badge, Button, Card, Dropdown, Empty, Input, List, Typography } from 'antd'
import type { MenuProps } from 'antd'
import {
  PlusOutlined,
  BellOutlined,
  DownOutlined,
  SearchOutlined,
  AppstoreOutlined,
  MessageOutlined,
  FileTextOutlined,
  CodeOutlined,
  CheckCircleOutlined,
  BuildOutlined,
  FolderOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useAuthStore } from '@/stores/auth'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/hooks/useProjectPermissions'
import { MatrixLogoLink } from '@/components/MatrixLogo'
import { avatarInitials } from '@/utils/avatar'
import * as notificationsApi from '@/api/notifications'
import type { Notification } from '@/api/notifications'
import { subscribeNotificationStream } from '@/api/stream'
import { formatRelativeTime } from '@/api/projects'
import './layouts.scss'

type ProjectNavIcon = 'overview' | 'chat' | 'plan' | 'implement' | 'verify' | 'build' | 'repository' | 'settings'

const projectNavIcons: Record<ProjectNavIcon, ComponentType<{ style?: CSSProperties; className?: string }>> = {
  overview: AppstoreOutlined,
  chat: MessageOutlined,
  plan: FileTextOutlined,
  implement: CodeOutlined,
  verify: CheckCircleOutlined,
  build: BuildOutlined,
  repository: FolderOutlined,
  settings: SettingOutlined,
}

export function AppShell() {
  const user = useAuthStore((s) => s.user)
  const isAdmin = useAuthStore((s) => s.user?.is_admin ?? false)
  const projects = useProjectStore((s) => s.projects)
  const currentProject = useProjectStore((s) => s.current)
  const fetchProjects = useProjectStore((s) => s.fetchProjects)
  const { id: projectId } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [projectPickerOpen, setProjectPickerOpen] = useState(false)
  const [notifyOpen, setNotifyOpen] = useState(false)
  const [unreadCount, setUnreadCount] = useState(0)
  const [notifications, setNotifications] = useState<Notification[]>([])

  const perms = useProjectPermissions(currentProject)
  const displayName = user?.name || user?.username

  const filteredProjects = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return projects
    return projects.filter((p) => p.name.toLowerCase().includes(q))
  }, [search, projects])

  const navItems = useMemo(() => {
    if (!projectId) return [] as const
    const base = `/projects/${projectId}`
    const items: { to: string; label: string; icon: ProjectNavIcon }[] = [
      { to: base, label: '概览', icon: 'overview' },
      { to: `${base}/chat`, label: '对话', icon: 'chat' },
      { to: `${base}/plan`, label: '编写计划', icon: 'plan' },
      { to: `${base}/implement`, label: '编码实现', icon: 'implement' },
      { to: `${base}/verify`, label: '验证评测', icon: 'verify' },
      { to: `${base}/build`, label: '执行构建', icon: 'build' },
      { to: `${base}/repository`, label: '仓库', icon: 'repository' },
    ]
    if (perms.canManageSettings) {
      items.push({ to: `${base}/-/settings/general`, label: '设置', icon: 'settings' })
    }
    return items
  }, [projectId, perms.canManageSettings])

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'header',
      type: 'group',
      label: (
        <div className="user-menu__header">
          <Avatar size={40} style={{ backgroundColor: '#fc6d26', flexShrink: 0 }}>
            {avatarInitials(displayName)}
          </Avatar>
          <div>
            <div className="user-menu__display">{displayName}</div>
            <div className="user-menu__username muted">@{user?.username}</div>
          </div>
        </div>
      ),
    },
    { type: 'divider' },
    { key: 'profile', label: '编辑资料', onClick: () => navigate('/profile') },
    ...(isAdmin ? [{ key: 'admin', label: '管理区域', onClick: () => navigate('/admin') }] : []),
    { type: 'divider' },
    { key: 'signout', label: '退出登录', danger: true, onClick: () => navigate('/users/sign_out') },
  ]

  useEffect(() => {
    if (!projects.length) {
      fetchProjects()
    }
  }, [projects.length, fetchProjects])

  useEffect(() => {
    setProjectPickerOpen(false)
    setNotifyOpen(false)
  }, [location.pathname])

  useEffect(() => {
    let cancelled = false

    async function loadNotifications() {
      if (!user) return
      try {
        const [countRes, listRes] = await Promise.all([
          notificationsApi.unreadCount(),
          notificationsApi.listNotifications(),
        ])
        if (!cancelled) {
          setUnreadCount(countRes.count)
          setNotifications(listRes.notifications)
        }
      } catch {
        /* ignore */
      }
    }

    loadNotifications()
    const timer = setInterval(loadNotifications, 30000)

    const unsubscribe = subscribeNotificationStream(() => {
      loadNotifications()
    })

    return () => {
      cancelled = true
      clearInterval(timer)
      unsubscribe()
    }
  }, [user])

  async function markAllRead() {
    await notificationsApi.markAllRead()
    setUnreadCount(0)
    setNotifications((prev) => prev.map((n) => ({ ...n, read_at: n.read_at || new Date().toISOString() })))
  }

  async function markRead(n: Notification) {
    if (!n.read_at) {
      await notificationsApi.markRead(n.id)
    }
    if (n.link) navigate(n.link)
    setNotifyOpen(false)
    const [countRes, listRes] = await Promise.all([
      notificationsApi.unreadCount(),
      notificationsApi.listNotifications(),
    ])
    setUnreadCount(countRes.count)
    setNotifications(listRes.notifications)
  }

  const projectMenuItems = useMemo((): MenuProps['items'] => {
    const items: MenuProps['items'] = filteredProjects.map((p) => ({
      key: p.id,
      icon: (
        <Avatar size={28} shape="square" style={{ backgroundColor: '#fc6d26', flexShrink: 0 }}>
          {avatarInitials(p.name)}
        </Avatar>
      ),
      label: p.name,
    }))

    if (!items.length) {
      items.push({ key: 'empty', label: '未找到项目', disabled: true })
    }

    items.push(
      { type: 'divider' },
      { key: 'new', label: '创建新项目' },
    )

    return items
  }, [filteredProjects])

  function onProjectMenuClick({ key }: { key: string }) {
    setProjectPickerOpen(false)
    if (key === 'new') {
      navigate('/projects/new')
    } else if (key !== 'empty') {
      navigate(`/projects/${key}`)
    }
  }

  function onProjectPickerOpenChange(open: boolean) {
    setProjectPickerOpen(open)
    if (!open) setSearch('')
  }

  return (
    <div className="app-shell">
      <header className="top-bar">
        <div className="top-bar__left">
          <MatrixLogoLink />
          <nav className="top-bar__nav">
            <Dropdown
              open={projectPickerOpen}
              onOpenChange={onProjectPickerOpenChange}
              destroyOnHidden
              placement="bottomLeft"
              trigger={['click']}
              menu={{
                items: projectMenuItems,
                onClick: onProjectMenuClick,
                style: { maxHeight: 280, overflow: 'auto', boxShadow: 'none', minWidth: 280 },
              }}
              popupRender={(menu) => (
                <div className="project-picker">
                  <div className="project-picker__search">
                    <Input
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      placeholder="搜索项目"
                      prefix={<SearchOutlined />}
                      allowClear
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => e.stopPropagation()}
                    />
                  </div>
                  {menu}
                </div>
              )}
            >
              <Button type="text" className="top-bar__nav-link">项目</Button>
            </Dropdown>
            <Link to="/groups" className="top-bar__nav-link">组</Link>
          </nav>
          {projectId && currentProject && (
            <div className="top-bar__breadcrumb">
              <span className="top-bar__sep">/</span>
              <Link to={`/projects/${projectId}`}>{currentProject.name}</Link>
            </div>
          )}
        </div>
        <div className="top-bar__right">
          <Dropdown
            open={notifyOpen}
            onOpenChange={setNotifyOpen}
            destroyOnHidden
            trigger={['click']}
            placement="bottomRight"
            popupRender={() => (
              <Card
                size="small"
                title="通知"
                extra={unreadCount > 0 ? (
                  <Button type="link" size="small" onClick={markAllRead}>
                    全部已读
                  </Button>
                ) : undefined}
                style={{ width: 320 }}
                styles={{ body: { maxHeight: 360, overflow: 'auto', padding: 0 } }}
              >
                {!notifications.length ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无通知" />
                ) : (
                  <List
                    size="small"
                    split
                    dataSource={notifications}
                    renderItem={(n) => (
                      <List.Item
                        onClick={() => markRead(n)}
                        style={{ cursor: 'pointer', paddingInline: 16 }}
                      >
                        <List.Item.Meta
                          title={<Typography.Text strong={!n.read_at}>{n.title}</Typography.Text>}
                          description={(
                            <>
                              <Typography.Paragraph style={{ marginBottom: 4 }}>{n.body}</Typography.Paragraph>
                              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                                {formatRelativeTime(n.created_at)}
                              </Typography.Text>
                            </>
                          )}
                        />
                      </List.Item>
                    )}
                  />
                )}
              </Card>
            )}
          >
            <Badge count={unreadCount} size="small">
              <Button
                type="text"
                aria-label="通知"
                title="通知"
                icon={<BellOutlined style={{ fontSize: 18 }} />}
              />
            </Badge>
          </Dropdown>
          <Button type="text" className="top-bar__action" title="新建项目" onClick={() => navigate('/projects/new')}>
            <PlusOutlined />
            <span className="top-bar__action-label">新建</span>
          </Button>
          <Dropdown
            menu={{ items: userMenuItems }}
            trigger={['click']}
            placement="bottomRight"
            destroyOnHidden
          >
            <Button type="text" className="user-menu__trigger">
              <Avatar size={26} style={{ backgroundColor: '#fc6d26', flexShrink: 0 }}>
                {avatarInitials(displayName)}
              </Avatar>
              <DownOutlined className="user-menu__chevron" style={{ fontSize: 10 }} />
            </Button>
          </Dropdown>
        </div>
      </header>
      <div className="app-shell__body">
        {projectId && currentProject && (
          <aside className="project-sidebar">
            <Link to={`/projects/${projectId}`} className="project-sidebar__head">
              <Avatar size={32} shape="square" style={{ backgroundColor: '#fc6d26', flexShrink: 0 }}>
                {avatarInitials(currentProject.name)}
              </Avatar>
              <span className="project-sidebar__name">{currentProject.name}</span>
            </Link>
            <nav className="project-sidebar__nav">
              {navItems.map((item) => {
                const Icon = projectNavIcons[item.icon]
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end={item.icon === 'overview'}
                    className={({ isActive }) => `project-sidebar__link${isActive ? ' nav-link-active' : ''}`}
                  >
                    <Icon className="matrix-nav-icon" style={{ fontSize: 16 }} />
                    <span>{item.label}</span>
                  </NavLink>
                )
              })}
            </nav>
          </aside>
        )}
        <main className="app-shell__main">
          <div className="app-shell__content page-container">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}
