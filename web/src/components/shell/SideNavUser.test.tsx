import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { SideNavUser } from './SideNavUser'

const mockAuth = { user: null as unknown, isAuthed: false, logout: vi.fn() }
vi.mock('../../auth/useAuth', () => ({ useAuth: () => mockAuth }))

function renderUser() {
  return render(<MemoryRouter><SideNavUser /></MemoryRouter>)
}

describe('SideNavUser', () => {
  it('未登录显示登录/注册', () => {
    mockAuth.user = null
    mockAuth.isAuthed = false
    renderUser()
    const link = screen.getByRole('link', { name: /登录|注册/ })
    expect(link).toHaveAttribute('href', '/login')
  })

  it('登录态显示昵称与积分', () => {
    mockAuth.user = { id: 1, nickname: '小明', credits: 42, avatarUrl: '' }
    mockAuth.isAuthed = true
    renderUser()
    expect(screen.getByText('小明')).toBeInTheDocument()
    expect(screen.getByText(/42/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /登录|注册/ })).not.toBeInTheDocument()
  })
})
