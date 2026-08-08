import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { User } from '../api/types'
import Profile from './Profile'

// 只测「邀请码卡片」按角色显隐：mock 掉路由、鉴权与 api，避免触网络与 Provider 依赖。
vi.mock('react-router-dom', () => ({
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}))

const getInviteCode = vi.fn()
const rotateInviteCode = vi.fn()
vi.mock('../api/client', () => ({
  api: {
    updateProfile: vi.fn(),
    uploadAvatar: vi.fn(),
    getInviteCode: () => getInviteCode(),
    rotateInviteCode: () => rotateInviteCode(),
  },
}))

let mockUser: User
vi.mock('../auth/useAuth', () => ({
  useAuth: () => ({ user: mockUser, refreshUser: vi.fn() }),
}))

function makeUser(role: 'admin' | 'user'): User {
  return {
    id: 1,
    username: 'u',
    email: 'u@e.com',
    role,
    nickname: '小明',
    age: 7,
    gender: '男',
    avatarUrl: '',
    credits: 10,
    createdAt: '2026-01-01',
  }
}

describe('Profile 邀请码卡片显隐', () => {
  beforeEach(() => {
    getInviteCode.mockReset().mockResolvedValue({ inviteCode: 'ABC123' })
    rotateInviteCode.mockReset().mockResolvedValue({ inviteCode: 'XYZ789' })
  })

  it('admin 显示邀请码卡片并请求邀请码', async () => {
    mockUser = makeUser('admin')
    render(<Profile />)
    expect(screen.getByText('邀请码 🎟️')).toBeInTheDocument()
    expect(getInviteCode).toHaveBeenCalledTimes(1)
    // 等异步拉码落地，避免 act 告警
    expect(await screen.findByText('ABC123')).toBeInTheDocument()
  })

  it('普通用户不渲染邀请码卡片且不请求', () => {
    mockUser = makeUser('user')
    render(<Profile />)
    expect(screen.queryByText('邀请码 🎟️')).toBeNull()
    expect(getInviteCode).not.toHaveBeenCalled()
  })
})
