import { describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import Login from './Login'

// 只测 UI 结构：切到注册 tab 后出现"邀请码"输入。
// 因此把路由与鉴权依赖 mock 掉，避免触网络与 Provider 依赖。
vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock('../auth/useAuth', () => ({
  useAuth: () => ({ login: vi.fn(), register: vi.fn() }),
}))

describe('Login', () => {
  it('默认登录 tab 不显示邀请码输入', () => {
    render(<Login />)
    expect(screen.queryByLabelText('邀请码')).toBeNull()
  })

  it('切到注册 tab 后出现邀请码输入', () => {
    render(<Login />)
    fireEvent.click(screen.getByText('注册'))
    expect(screen.getByLabelText('邀请码')).toBeInTheDocument()
    expect(screen.getByLabelText('邮箱')).toBeInTheDocument()
    expect(screen.getByLabelText(/昵称/)).toBeInTheDocument()
  })
})
