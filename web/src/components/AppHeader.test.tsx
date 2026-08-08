import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { AppHeader } from './AppHeader'
import { AuthProvider } from '../auth/useAuth'
import type { User } from '../api/types'

// AppHeader 依赖 AuthProvider（真实 Provider），Provider 又依赖 api/client。
// mock 掉 client：初始态从 localStorage 读取，故通过预置 localStorage 控制登录态。
vi.mock('../api/client', () => ({
  api: {
    login: vi.fn(),
    register: vi.fn(),
    getMe: vi.fn(),
    post: vi.fn(),
  },
  setUnauthorizedHandler: vi.fn(),
}))

const STORAGE_KEY = 'omc.auth'

function makeUser(): User {
  return {
    id: 1,
    username: 'u',
    email: 'u@e.com',
    role: 'user',
    nickname: '小明',
    age: 7,
    gender: '男',
    avatarUrl: '',
    credits: 42,
    createdAt: '2026-01-01',
  }
}

function renderHeader() {
  return render(
    <MemoryRouter>
      <AuthProvider>
        <AppHeader />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('AppHeader 登录态分支', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('未登录：渲染登录/注册链接且不渲染用户菜单', () => {
    renderHeader()
    const link = screen.getByRole('link', { name: '登录 / 注册' })
    expect(link).toHaveAttribute('href', '/login')
    expect(screen.queryByRole('button', { name: '用户菜单' })).toBeNull()
    expect(screen.queryByText(/积分/)).toBeNull()
  })

  it('已登录：渲染积分与用户菜单，不渲染登录/注册链接', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ authed: true, user: makeUser() }))
    renderHeader()
    expect(screen.getByRole('button', { name: '用户菜单' })).toBeInTheDocument()
    expect(screen.getByText(/积分 42/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '登录 / 注册' })).toBeNull()
  })
})
