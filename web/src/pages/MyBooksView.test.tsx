import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../api/client', () => ({
  setUnauthorizedHandler: vi.fn(),
  api: {
    get: vi.fn().mockResolvedValue([
      {
        id: 1,
        userId: 1,
        title: '书A',
        coverUrl: '',
        style: 'ghibli',
        summary: '',
        isPublic: false,
        createdAt: 't',
        updatedAt: 't',
        likeCount: 0,
        viewCount: 0,
        publishedAt: '',
      },
    ]),
    setVisibility: vi.fn().mockResolvedValue({
      id: 1,
      userId: 1,
      title: '书A',
      coverUrl: '',
      style: 'ghibli',
      summary: '',
      isPublic: true,
      createdAt: 't',
      updatedAt: 't',
      likeCount: 0,
      viewCount: 0,
      publishedAt: 't2',
    }),
  },
}))
import { api } from '../api/client'
import MyBooksView from './MyBooksView'

describe('MyBooksView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('加载书籍并可切换公开', async () => {
    render(
      <MemoryRouter>
        <MyBooksView />
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getAllByText('书A').length).toBeGreaterThan(0))
    fireEvent.click(screen.getByRole('button', { name: /公开|发布/ }))
    await waitFor(() => expect(api.setVisibility).toHaveBeenCalledWith(1, true))
  })
})
