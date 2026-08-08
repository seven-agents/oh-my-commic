import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { CommunityCard } from './CommunityCard'
import type { CommunityBook } from '../api/types'

const book: CommunityBook = {
  id: 5, title: '小熊的一天', coverUrl: '', summary: '温暖的故事',
  author: { nickname: '小明', avatarUrl: '' },
  likeCount: 3, viewCount: 12, liked: false, publishedAt: 't',
}

describe('CommunityCard', () => {
  it('展示标题/概述/作者/点赞与浏览数，链接到阅读页', () => {
    render(<MemoryRouter><CommunityCard book={book} /></MemoryRouter>)
    expect(screen.getByText('小熊的一天')).toBeInTheDocument()
    expect(screen.getByText('温暖的故事')).toBeInTheDocument()
    expect(screen.getByText('小明')).toBeInTheDocument()
    expect(screen.getByText(/3/)).toBeInTheDocument()
    expect(screen.getByText(/12/)).toBeInTheDocument()
    expect(screen.getByRole('link')).toHaveAttribute('href', '/community/books/5')
  })
})
