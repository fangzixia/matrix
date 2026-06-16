import type { NavigationGuardNext, RouteLocationNormalized } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

/** 登录成功后的安全跳转路径：根路径或未指定时进项目列表 */
export function resolvePostLoginRedirect(redirect?: string | null): string {
  const path = (redirect ?? '').trim()
  if (!path || path === '/') {
    return '/projects'
  }
  return path
}

export async function authGuard(
  to: RouteLocationNormalized,
  _from: RouteLocationNormalized,
  next: NavigationGuardNext,
) {
  const auth = useAuthStore()
  if (!auth.initialized) {
    await auth.fetchMe()
  }
  if (to.meta.public) {
    if (auth.isLoggedIn && to.name === 'sign-in') {
      next({ path: '/projects' })
      return
    }
    next()
    return
  }
  if (!auth.isLoggedIn) {
    next({ name: 'sign-in', query: { redirect: resolvePostLoginRedirect(to.fullPath) } })
    return
  }  if (to.meta.admin && !auth.isAdmin) {
    next({ name: 'forbidden' })
    return
  }
  if (to.meta.root && !auth.isRoot) {
    next({ name: 'forbidden' })
    return
  }
  next()
}
