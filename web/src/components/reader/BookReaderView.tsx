import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { AppHeader } from '../AppHeader'
import { Button, EmptyState, LoadingClouds } from '../ui'
import { ReaderPage, type ReaderPageData } from './ReaderPage'

interface BookReaderViewProps {
  loading: boolean
  error: string
  title: string
  coverPage: ReaderPageData | null // kind:'cover'
  chapterPages: ReaderPageData[] // kind:'chapter'
  headerRight?: ReactNode // 头部右侧插槽
  footer?: ReactNode // 底部插槽(点赞栏等)
}

// 阅读器纯展示层：翻页 state + 键盘左右 + 封面/章节页拼装渲染 + loading/error/空态。
// 不做任何数据获取，供 owner 私有阅读与社区公开阅读共用。
export function BookReaderView({
  loading,
  error,
  // title 属于共享契约（供社区页/文档标题等复用），当前展示层的标题取自 coverPage，暂不消费。
  coverPage,
  chapterPages,
  headerRight,
  footer,
}: BookReaderViewProps) {
  const [index, setIndex] = useState(0)

  const pages = useMemo<ReaderPageData[]>(
    () => (coverPage ? [coverPage, ...chapterPages] : chapterPages),
    [coverPage, chapterPages],
  )

  const total = pages.length
  const goPrev = useCallback(() => setIndex((i) => Math.max(0, i - 1)), [])
  const goNext = useCallback(() => setIndex((i) => Math.min(total - 1, i + 1)), [total])

  // 有可翻页面（封面或章节）就渲染阅读器并监听键盘。
  const hasReadablePages = pages.length > 0

  useEffect(() => {
    if (!hasReadablePages) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft') goPrev()
      else if (e.key === 'ArrowRight') goNext()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [hasReadablePages, goPrev, goNext])

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader right={headerRight} />

      <main className="mx-auto max-w-4xl px-6 py-8">
        {loading ? (
          <LoadingClouds label="正在翻开这本书…" />
        ) : error || !coverPage ? (
          <EmptyState
            emoji="🌙"
            title="这本书还没准备好"
            description={error || '找不到这本书'}
            action={
              <Link to="/my">
                <Button variant="ghost">回到书架</Button>
              </Link>
            }
          />
        ) : !hasReadablePages ? (
          <EmptyState
            emoji="🎨"
            title="这本书还没画好的漫画"
            description="先去编辑画几页吧～"
            action={headerRight}
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
            {footer}
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
