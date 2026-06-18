import { Outlet } from 'react-router-dom'
import { MatrixLogo } from '@/components/MatrixLogo'
import './auth-layout.scss'

export function AuthLayout() {
  return (
    <div className="auth-layout">
      <header className="auth-layout__header">
        <MatrixLogo />
      </header>
      <main className="auth-layout__main">
        <div className="auth-layout__grid">
          <section className="auth-layout__hero">
            <div className="auth-layout__hero-badge">自托管</div>
            <h1>完整的 AI 交付平台</h1>
            <p className="auth-layout__lead">
              Matrix 将需求、源码、AI 运行与评测整合于单一应用，覆盖从规划到验证的全流程。
            </p>
            <p className="auth-layout__hint muted">这是您自托管的 Matrix 实例。</p>
          </section>
          <section className="auth-layout__card">
            <Outlet />
          </section>
        </div>
      </main>
      <footer className="auth-layout__footer muted">Matrix</footer>
    </div>
  )
}
