import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, setUnauthorizedHandler } from '../api/client'
import type { User } from '../api/types'

const STORAGE_KEY = 'omc.auth'

type StoredAuth = { authed: boolean; user: User | null }

function loadStored(): StoredAuth {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { authed: false, user: null }
    const parsed = JSON.parse(raw) as StoredAuth
    return { authed: !!parsed.authed, user: parsed.user ?? null }
  } catch {
    return { authed: false, user: null }
  }
}

function persist(state: StoredAuth) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // 忽略存储失败（隐私模式等）
  }
}

function clearStored() {
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // 忽略
  }
}

// 注册入参：用户名 + 密码 + 邮箱 + 邀请码，昵称可选（缺省时后端回退到用户名）。
type RegisterInput = {
  username: string
  password: string
  email: string
  inviteCode: string
  nickname?: string
}

type AuthContextValue = {
  user: User | null
  isAuthed: boolean
  login: (username: string, password: string) => Promise<void>
  register: (input: RegisterInput) => Promise<void>
  logout: () => Promise<void>
  // 从 /api/v1/me 拉取最新用户（含积分余额）并更新本地态，供出图/漫画化后刷新 header。
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const initial = loadStored()
  const [user, setUser] = useState<User | null>(initial.user)
  const [isAuthed, setIsAuthed] = useState<boolean>(initial.authed)

  const applyLogout = useCallback(() => {
    setUser(null)
    setIsAuthed(false)
    clearStored()
  }, [])

  const login = useCallback(async (username: string, password: string) => {
    const u = await api.login({ username, password })
    setUser(u)
    setIsAuthed(true)
    persist({ authed: true, user: u })
  }, [])

  const register = useCallback(async (input: RegisterInput) => {
    // 注册成功后后端已 set cookie 自动登录，直接用返回的用户落地登录态
    const u = await api.register(input)
    setUser(u)
    setIsAuthed(true)
    persist({ authed: true, user: u })
  }, [])

  // 刷新当前用户（积分等）。静默失败：出图/漫画化本身已成功，余额展示滞后无妨；
  // 若会话失效返回 401，client 的全局处理会触发登出。
  const refreshUser = useCallback(async () => {
    try {
      const me = await api.getMe()
      setUser(me)
      setIsAuthed(true)
      persist({ authed: true, user: me })
    } catch {
      // 忽略刷新失败，保留现有展示
    }
  }, [])

  const logout = useCallback(async () => {
    try {
      await api.post<void>('/api/v1/logout')
    } catch {
      // 即使后端登出失败也清空本地态
    }
    applyLogout()
  }, [applyLogout])

  // 注册全局 401 处理：任意 API 返回 401 时清空登录态。
  // RequireAuth 会据此把用户带回 /login。
  useEffect(() => {
    setUnauthorizedHandler(applyLogout)
    return () => setUnauthorizedHandler(null)
  }, [applyLogout])

  const value = useMemo<AuthContextValue>(
    () => ({ user, isAuthed, login, register, logout, refreshUser }),
    [user, isAuthed, login, register, logout, refreshUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth 必须在 AuthProvider 内使用')
  return ctx
}
