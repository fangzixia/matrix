import { Link } from 'react-router-dom'

export default function NotFoundPage() {
  return (
    <div className="panel not-found" style={{ maxWidth: 480, margin: '48px auto', textAlign: 'center' }}>
      <h1 className="page-title">404</h1>
      <p className="muted">页面不存在。</p>
      <Link to="/projects">返回项目列表</Link>
    </div>
  )
}
