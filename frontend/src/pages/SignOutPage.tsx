import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'

export default function SignOutPage() {
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function run() {
      try {
        await logout()
      } finally {
        setLoading(false)
        navigate('/users/sign_in', { replace: true })
      }
    }
    run()
  }, [logout, navigate])

  return <p className="muted">{loading ? '退出中…' : '正在跳转…'}</p>
}
