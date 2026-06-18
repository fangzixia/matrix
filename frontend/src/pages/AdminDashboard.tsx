import { Link } from 'react-router-dom'

export default function AdminDashboard() {
  return (
    <div>
      <h1 className="page-title">管理概览</h1>
      <div className="panel">
        <p style={{ margin: '0 0 12px', lineHeight: 1.6, color: 'var(--matrix-text-color-subtle)' }}>
          欢迎使用 Matrix 管理区域。在此管理用户、查看实例设置并监控平台访问。
        </p>
        <Link to="/admin/users" style={{ fontWeight: 600, fontSize: 14 }}>前往用户管理 →</Link>
      </div>
    </div>
  )
}
