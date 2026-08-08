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
      likeCount: 3,
      viewCount: 5,
      publishedAt: 't2',
    }),
  },
}))
import { api } from '../api/client'
import Bookshelf from './Bookshelf'
import { AuthProvider } from '../auth/useAuth'

describe('Bookshelf 公开开关', () => {
  beforeEach(() => vi.clearAllMocks())

  it('点击开关调用 setVisibility', async () => {
    render(
      <MemoryRouter>
        <AuthProvider>
          <Bookshelf />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getAllByText('书A').length).toBeGreaterThan(0))
    const toggle = screen.getByRole('button', { name: /公开|发布/ })
    fireEvent.click(toggle)
    await waitFor(() => expect(api.setVisibility).toHaveBeenCalledWith(1, true))
  })

  it('切换后展示计数与公开徽标', async () => {
    render(
      <MemoryRouter>
        <AuthProvider>
          <Bookshelf />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getAllByText('书A').length).toBeGreaterThan(0))
    fireEvent.click(screen.getByRole('button', { name: /公开|发布/ }))
    await waitFor(() => expect(screen.getByText(/❤/)).toBeInTheDocument())
    expect(screen.getByText(/👁/)).toBeInTheDocument()
  })
})
