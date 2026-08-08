import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { FeaturedRow } from './FeaturedRow'
import type { CommunityBook } from '../../api/types'

const book = (id: number, title: string): CommunityBook => ({
  id, title, coverUrl: '', summary: '梗概',
  author: { nickname: '小明', avatarUrl: '' },
  likeCount: 9, viewCount: 3, liked: false, publishedAt: 't',
})

describe('FeaturedRow', () => {
  it('渲染每本精选书的标题与阅读链接', () => {
    render(
      <MemoryRouter>
        <FeaturedRow books={[book(1, '甲'), book(2, '乙')]} />
      </MemoryRouter>,
    )
    expect(screen.getByText('甲')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /乙/ })).toHaveAttribute('href', '/community/books/2')
  })
})
