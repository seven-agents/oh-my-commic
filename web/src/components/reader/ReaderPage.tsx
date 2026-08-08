import { Card } from '../ui'
import { ComicPage } from '../ComicPage'
import { BookCover } from '../BookCover'
import { mediaUrl } from '../../api/media'
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
    const cover = mediaUrl(page.coverUrl)
    return (
      <article className="animate-pop-in flex flex-col items-center gap-6">
        {/* 封面做成和后面章节页一样的大尺度，翻页时不再突兀。 */}
        <Card className="w-full bg-parchment p-3 sm:p-4">
          {cover ? (
            <img
              src={cover}
              alt={page.title}
              className="mx-auto max-h-[72vh] w-full rounded-2xl object-contain"
              loading="lazy"
            />
          ) : (
            <div className="mx-auto w-full max-w-sm">
              <BookCover id={page.bookId} title={page.title} coverUrl={page.coverUrl} />
            </div>
          )}
        </Card>
        <h1 className="text-center font-display text-3xl font-extrabold text-ink">{page.title}</h1>
        {page.summary && (
          <Card className="max-w-2xl bg-white/70">
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
