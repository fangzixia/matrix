import { useEffect } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { Spin } from 'antd'
import { useAuthStore } from '@/stores/auth'
import { resolvePostLoginRedirect } from './guards'

interface AuthGuardProps {
  public?: boolean
  admin?: boolean
  root?: boolean
}

export function AuthGuard({ public: isPublic, admin, root }: AuthGuardProps) {
  const initialized = useAuthStore((s) => s.initialized)
  const user = useAuthStore((s) => s.user)
  const fetchMe = useAuthStore((s) => s.fetchMe)
  const location = useLocation()

  useEffect(() => {
    if (!initialized) {
      fetchMe()
    }
  }, [initialized, fetchMe])

  if (!initialized) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 48 }}>
        <Spin size="large" />
      </div>
    )
  }

  if (isPublic) {
    if (user && location.pathname === '/users/sign_in') {
      return <Navigate to="/projects" replace />
    }
    return <Outlet />
  }

  if (!user) {
    const redirect = resolvePostLoginRedirect(location.pathname + location.search)
    return <Navigate to={`/users/sign_in?redirect=${encodeURIComponent(redirect)}`} replace />
  }

  if (admin && !user.is_admin) {
    return <Navigate to="/403" replace />
  }

  const isRoot = user.is_root || user.username === 'root'
  if (root && !isRoot) {
    return <Navigate to="/403" replace />
  }

  return <Outlet />
}
