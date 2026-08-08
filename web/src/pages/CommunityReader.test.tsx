import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'

vi.mock('../api/client', () => ({
  api: {
    getCommunityBook: vi.fn().mockResolvedValue({
      id: 5,
      title: '小熊的一天',
      coverUrl: '',
      summary: '温暖',
      style: 'ghibli',
      author: { nickname: '小明', avatarUrl: '' },
      likeCount: 2,
      viewCount: 9,
      liked: false,
      chapters: [],
    }),
    recordView: vi.fn().mockResolvedValue({ ok: true }),
    likeBook: vi.fn(),
    unlikeBook: vi.fn(),
  },
  // AuthProvider 挂载时会注册全局 401 处理，mock 需保留该导出。
  setUnauthorizedHandler: vi.fn(),
}))
import { api } from '../api/client'
import CommunityReader from './CommunityReader'
import { AuthProvider } from '../auth/useAuth'

describe('CommunityReader', () => {
  beforeEach(() => vi.clearAllMocks())

  it('加载详情并记一次浏览', async () => {
    render(
      <MemoryRouter initialEntries={['/community/books/5']}>
        <AuthProvider>
          <Routes>
            <Route path="/community/books/:id" element={<CommunityReader />} />
          </Routes>
        </AuthProvider>
      </MemoryRouter>,
    )
    // 无封面图时 ReaderPage 会在标题 h1 与 BookCover 占位里各渲染一次标题，用 getAllByText。
    await waitFor(() => expect(screen.getAllByText('小熊的一天').length).toBeGreaterThan(0))
    expect(api.getCommunityBook).toHaveBeenCalledWith(5)
    expect(api.recordView).toHaveBeenCalledTimes(1)
  })
})
