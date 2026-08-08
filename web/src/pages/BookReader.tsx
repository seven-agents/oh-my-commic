import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '../components/ui'
import { BookReaderView } from '../components/reader/BookReaderView'
import type { ReaderPageData } from '../components/reader/ReaderPage'
import { api } from '../api/client'
import type { Book, Chapter, Panel } from '../api/types'
import { errorMessage } from '../api/errors'

// 判断一章是否有已渲染（有图）的分镜。
function hasRenderedPanels(panels: Panel[]): boolean {
  return panels.some((p) => p.status === 'done' && Boolean(p.imageUrl))
}

// 整本书翻页阅读器（owner 私有）：数据获取 + 组装页面，渲染交给 BookReaderView。
export default function BookReader() {
  const { id } = useParams<{ id: string }>()
  const bookId = id ?? ''

  const [book, setBook] = useState<Book | null>(null)
  const [chapterPages, setChapterPages] = useState<ReaderPageData[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!bookId) return
    let alive = true
    ;(async () => {
      setLoading(true)
      setError('')
      try {
        const [bk, chapters] = await Promise.all([
          api.get<Book>(`/api/v1/books/${bookId}`),
          api.get<Chapter[]>(`/api/v1/books/${bookId}/chapters`),
        ])
        // 封面章由封面页(page 0)代表，不再单独成一页。
        const ordered = [...(chapters ?? [])]
          .filter((ch) => !ch.isCover)
          .sort((a, b) => a.order - b.order)
        const panelLists = await Promise.all(
          ordered.map((ch) => api.get<Panel[]>(`/api/v1/chapters/${ch.id}/panels`)),
        )
        if (!alive) return

        const pages: ReaderPageData[] = ordered
          .map((ch, i) => ({ ch, panels: panelLists[i] ?? [] }))
          .filter(({ panels }) => hasRenderedPanels(panels))
          .map(({ ch, panels }) => ({
            kind: 'chapter' as const,
            chapterTitle: ch.title,
            panels,
            summary: ch.summary ?? '',
          }))

        setBook(bk)
        setChapterPages(pages)
      } catch (err) {
        if (alive) setError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [bookId])

  const coverPage = useMemo<ReaderPageData | null>(() => {
    if (!book) return null
    return {
      kind: 'cover',
      bookId: book.id,
      title: book.title,
      coverUrl: book.coverUrl,
      summary: book.summary ?? '',
    }
  }, [book])

  const editLink = `/books/${bookId}`

  return (
    <BookReaderView
      loading={loading}
      error={error}
      title={book?.title ?? ''}
      coverPage={coverPage}
      chapterPages={chapterPages}
      headerRight={
        <div className="flex items-center gap-2">
          <Link to="/my">
            <Button variant="ghost" className="text-sm">
              ← 返回书架
            </Button>
          </Link>
          <Link to={editLink}>
            <Button variant="ghost" className="text-sm">
              ✏️ 去编辑
            </Button>
          </Link>
        </div>
      }
    />
  )
}
