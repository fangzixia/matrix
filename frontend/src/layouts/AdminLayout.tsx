import { Link, NavLink, Outlet } from 'react-router-dom'
import { DashboardOutlined, LeftOutlined, SettingOutlined, UserOutlined } from '@ant-design/icons'
import { MatrixLogo } from '@/components/MatrixLogo'
import { useAuthStore } from '@/stores/auth'
import './admin-layout.scss'

export function AdminLayout() {
  const isRoot = useAuthStore((s) => s.isRoot())

  return (
    <div className="admin-layout">
      <header className="admin-layout__header">
        <Link to="/projects" className="admin-layout__back">
          <LeftOutlined /> 返回应用
        </Link>
        <MatrixLogo showText={false} size={20} />
        <span className="admin-layout__title">管理区域</span>
      </header>
      <div className="admin-layout__body">
        <aside className="admin-layout__sidebar">
          <p className="admin-layout__section">管理</p>
          <nav className="admin-layout__nav">
            <NavLink to="/admin" end className={({ isActive }) => (isActive ? 'nav-link-active' : undefined)}>
              <DashboardOutlined className="matrix-nav-icon" style={{ fontSize: 16 }} /> 概览
            </NavLink>
            <NavLink to="/admin/users" className={({ isActive }) => (isActive ? 'nav-link-active' : undefined)}>
              <UserOutlined className="matrix-nav-icon" style={{ fontSize: 16 }} /> 用户
            </NavLink>
            {isRoot && (
              <NavLink to="/admin/system" className={({ isActive }) => (isActive ? 'nav-link-active' : undefined)}>
                <SettingOutlined className="matrix-nav-icon" style={{ fontSize: 16 }} /> 系统配置
              </NavLink>
            )}
          </nav>
        </aside>
        <section className="admin-layout__content">
          <Outlet />
        </section>
      </div>
    </div>
  )
}
