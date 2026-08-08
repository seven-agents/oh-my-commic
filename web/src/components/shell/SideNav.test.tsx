import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { SideNav } from './SideNav'

vi.mock('../../auth/useAuth', () => ({
  useAuth: () => ({ user: null, isAuthed: false, logout: vi.fn() }),
}))

describe('SideNav', () => {
  it('渲染社区与我的漫画两个 tab，指向正确路由', () => {
    render(<MemoryRouter><SideNav /></MemoryRouter>)
    expect(screen.getByRole('link', { name: /社区/ })).toHaveAttribute('href', '/community')
    expect(screen.getByRole('link', { name: /我的漫画/ })).toHaveAttribute('href', '/my')
  })
})
