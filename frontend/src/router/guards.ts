/** 登录成功后的安全跳转路径 */
export function resolvePostLoginRedirect(redirect?: string | null): string {
  const path = (redirect ?? '').trim()
  if (!path || path === '/') {
    return '/projects'
  }
  return path
}
