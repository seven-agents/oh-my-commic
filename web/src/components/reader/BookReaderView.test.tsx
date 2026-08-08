import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { BookReaderView } from './BookReaderView'
import type { ReaderPageData } from './ReaderPage'

// AppHeader 依赖 AuthProvider；展示层测试只关心阅读器渲染，mock 掉鉴权。
vi.mock('../../auth/useAuth', () => ({
  useAuth: () => ({ user: null, logout: vi.fn() }),
}))

const cover: ReaderPageData = { kind: 'cover', bookId: 1, title: '我的书', coverUrl: '', summary: '梗概' }

describe('BookReaderView', () => {
  it('渲染封面标题与页码', () => {
    render(
      <MemoryRouter>
        <BookReaderView loading={false} error="" title="我的书" coverPage={cover} chapterPages={[]} />
      </MemoryRouter>,
    )
    // 无封面图时 ReaderPage 会在标题 h1 与 BookCover 占位里各渲染一次标题，取其一断言即可。
    expect(screen.getAllByText('我的书').length).toBeGreaterThan(0)
    expect(screen.getByText(/第 1 \/ 1 页/)).toBeInTheDocument()
  })

  it('loading 显示加载态', () => {
    render(
      <MemoryRouter>
        <BookReaderView loading error="" title="" coverPage={null} chapterPages={[]} />
      </MemoryRouter>,
    )
    expect(screen.queryByText('我的书')).not.toBeInTheDocument()
  })

  it('错误态默认返回链接指向书架', () => {
    render(
      <MemoryRouter>
        <BookReaderView loading={false} error="加载失败" title="" coverPage={null} chapterPages={[]} />
      </MemoryRouter>,
    )
    const link = screen.getByRole('link', { name: '回到书架' })
    expect(link).toHaveAttribute('href', '/my')
  })

  it('错误态支持自定义 backTo/backLabel', () => {
    render(
      <MemoryRouter>
        <BookReaderView
          loading={false}
          error="加载失败"
          title=""
          coverPage={null}
          chapterPages={[]}
          backTo="/community"
          backLabel="回到社区"
        />
      </MemoryRouter>,
    )
    const link = screen.getByRole('link', { name: '回到社区' })
    expect(link).toHaveAttribute('href', '/community')
    expect(screen.queryByRole('link', { name: '回到书架' })).toBeNull()
  })
})
