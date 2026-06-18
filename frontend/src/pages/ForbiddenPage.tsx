import { Link } from 'react-router-dom'

export default function ForbiddenPage() {
  return (
    <div className="panel">
      <h2>403 Forbidden</h2>
      <p className="muted">您没有权限访问此页面。</p>
      <Link to="/projects">返回项目列表</Link>
    </div>
  )
}
