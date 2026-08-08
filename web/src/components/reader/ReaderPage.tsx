import { Card } from '../ui'
import { ComicPage } from '../ComicPage'
import { BookCover } from '../BookCover'
import type { Panel } from '../../api/types'

// 阅读器里的一页：封面页或章节页，二选一。
export type ReaderPageData =
  | {
      kind: 'cover'
      bookId: number
      title: string
      coverUrl: string
      summary: string
    }
  | {
      kind: 'chapter'
      chapterTitle: string
      panels: Panel[]
      summary: string
    }

// 单页渲染，带轻微翻页淡入动画（key 变化触发）。
export function ReaderPage({ page }: { page: ReaderPageData }) {
  if (page.kind === 'cover') {
    return (
      <article className="animate-pop-in flex flex-col items-center gap-6">
        <div className="w-56 sm:w-64">
          <BookCover id={page.bookId} title={page.title} coverUrl={page.coverUrl} />
        </div>
        <h1 className="text-center font-display text-3xl font-extrabold text-ink">{page.title}</h1>
        {page.summary && (
          <Card className="max-w-xl bg-parchment">
            <p className="font-body leading-relaxed text-ink">{page.summary}</p>
          </Card>
        )}
      </article>
    )
  }

  return (
    <article className="animate-pop-in flex flex-col gap-6">
      <header className="text-center">
        <h2 className="font-display text-2xl font-extrabold text-ink">{page.chapterTitle}</h2>
      </header>
      <Card className="bg-parchment">
        <ComicPage panels={page.panels} />
      </Card>
      {page.summary && (
        <Card className="bg-white/70">
          <p className="font-body leading-relaxed text-ink">{page.summary}</p>
        </Card>
      )}
    </article>
  )
}
