import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../api/client', () => ({
  api: { listCommunity: vi.fn() },
}))
import { api } from '../api/client'
import CommunityView from './CommunityView'
import type { CommunityBook } from '../api/types'

const mk = (id: number, title: string, like = 0): CommunityBook => ({
  id, title, coverUrl: '', summary: 's',
  author: { nickname: '小明', avatarUrl: '' },
  likeCount: like, viewCount: 0, liked: false, publishedAt: 't',
})

describe('CommunityView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('精选(最热3)+网格去重：网格不重复渲染精选书', async () => {
    const featured = [mk(1, '热一', 9), mk(2, '热二', 8), mk(3, '热三', 7)]
    const grid = [mk(1, '热一', 9), mk(2, '热二', 8), mk(3, '热三', 7), mk(4, '普通四')]
    ;(api.listCommunity as unknown as ReturnType<typeof vi.fn>).mockImplementation(
      (opts: { sort?: string; limit?: number }) =>
        Promise.resolve(opts?.sort === 'hot' && opts?.limit === 3 ? featured : grid),
    )
    render(<MemoryRouter><CommunityView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('普通四')).toBeInTheDocument())
    // 精选区标题存在（说明精选栏渲染）
    expect(screen.getByText(/精选/)).toBeInTheDocument()
    // “热一” 只出现在精选，不在网格重复 → 页面中仅 1 处
    expect(screen.getAllByText('热一')).toHaveLength(1)
  })

  it('公开书≤3 时不渲染精选栏', async () => {
    const three = [mk(1, 'A'), mk(2, 'B'), mk(3, 'C')]
    ;(api.listCommunity as unknown as ReturnType<typeof vi.fn>).mockImplementation(
      (opts: { limit?: number }) => Promise.resolve(opts?.limit === 3 ? three : three),
    )
    render(<MemoryRouter><CommunityView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('A')).toBeInTheDocument())
    expect(screen.queryByText(/精选/)).not.toBeInTheDocument()
  })

  it('切换到最热重拉网格', async () => {
    ;(api.listCommunity as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([mk(1, '甲')])
    render(<MemoryRouter><CommunityView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('甲')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /最热/ }))
    await waitFor(() =>
      expect(api.listCommunity).toHaveBeenCalledWith(
        expect.objectContaining({ sort: 'hot', offset: 0 }),
      ),
    )
  })
})
