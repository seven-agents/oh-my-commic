import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { Button, EmptyState, LoadingClouds } from '../components/ui'
import { ReaderPage, type ReaderPageData } from '../components/reader/ReaderPage'
import { api } from '../api/client'
import type { Book, Chapter, Panel } from '../api/types'
import { errorMessage } from '../api/errors'

// 判断一章是否有已渲染（有图）的分镜。
function hasRenderedPanels(panels: Panel[]): boolean {
  return panels.some((p) => p.status === 'done' && Boolean(p.imageUrl))
}

// 整本书翻页阅读器：封面页 + 每个有画好分镜的章节一页。
export default function BookReader() {
  const { id } = useParams<{ id: string }>()
  const bookId = id ?? ''

  const [book, setBook] = useState<Book | null>(null)
  const [chapterPages, setChapterPages] = useState<ReaderPageData[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [index, setIndex] = useState(0)

  useEffect(() => {
    if (!bookId) return
    let alive = true
    ;(async () => {
      setLoading(true)
      setError('')
      try {
        const [bk, chapters] = await Promise.all([
          api.get<Book>(`/api/books/${bookId}`),
          api.get<Chapter[]>(`/api/books/${bookId}/chapters`),
        ])
        // 封面章由封面页(page 0)代表，不再单独成一页。
        const ordered = [...(chapters ?? [])]
          .filter((ch) => !ch.isCover)
          .sort((a, b) => a.order - b.order)
        const panelLists = await Promise.all(
          ordered.map((ch) => api.get<Panel[]>(`/api/chapters/${ch.id}/panels`)),
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

  const pages = useMemo<ReaderPageData[]>(() => {
    if (!book) return []
    const cover: ReaderPageData = {
      kind: 'cover',
      bookId: book.id,
      title: book.title,
      coverUrl: book.coverUrl,
      summary: book.summary ?? '',
    }
    return [cover, ...chapterPages]
  }, [book, chapterPages])

  const total = pages.length
  const goPrev = useCallback(() => setIndex((i) => Math.max(0, i - 1)), [])
  const goNext = useCallback(() => setIndex((i) => Math.min(total - 1, i + 1)), [total])

  const hasReadablePages = chapterPages.length > 0

  useEffect(() => {
    if (!hasReadablePages) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft') goPrev()
      else if (e.key === 'ArrowRight') goNext()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [hasReadablePages, goPrev, goNext])

  const editLink = `/books/${bookId}`

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader
        right={
          <div className="flex items-center gap-2">
            <Link to="/">
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

      <main className="mx-auto max-w-4xl px-6 py-8">
        {loading ? (
          <LoadingClouds label="正在翻开这本书…" />
        ) : error || !book ? (
          <EmptyState
            emoji="🌙"
            title="这本书还没准备好"
            description={error || '找不到这本书'}
            action={
              <Link to="/">
                <Button variant="ghost">回到书架</Button>
              </Link>
            }
          />
        ) : !hasReadablePages ? (
          <EmptyState
            emoji="🎨"
            title="这本书还没画好的漫画"
            description="先去编辑画几页吧～"
            action={
              <Link to={editLink}>
                <Button>✏️ 去编辑</Button>
              </Link>
            }
          />
        ) : (
          <div className="flex flex-col gap-6">
            <div className="flex items-start gap-3 sm:gap-5">
              <FlipButton dir="prev" onClick={goPrev} disabled={index === 0} />
              <div className="min-w-0 flex-1">
                <ReaderPage key={index} page={pages[index]} />
              </div>
              <FlipButton dir="next" onClick={goNext} disabled={index >= total - 1} />
            </div>
            <p className="text-center font-display font-semibold text-ink-soft">
              第 {index + 1} / {total} 页
            </p>
          </div>
        )}
      </main>
    </div>
  )
}

function FlipButton({
  dir,
  onClick,
  disabled,
}: {
  dir: 'prev' | 'next'
  onClick: () => void
  disabled: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={dir === 'prev' ? '上一页' : '下一页'}
      className="sticky top-24 flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-white font-display text-2xl font-bold text-ink shadow-soft-sm transition-all hover:-translate-y-0.5 hover:text-coral disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:translate-y-0 disabled:hover:text-ink"
    >
      <span aria-hidden>{dir === 'prev' ? '‹' : '›'}</span>
    </button>
  )
}
