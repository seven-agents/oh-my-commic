import { Navigate, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useAuth } from './useAuth'

// 未登录时重定向到 /login。由于没有 me 接口，
// 有本地 flag 时乐观地视为已登录；任意 API 的 401 会清空登录态并把用户带回登录页。
export function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthed } = useAuth()
  const location = useLocation()

  if (!isAuthed) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }
  return <>{children}</>
}
