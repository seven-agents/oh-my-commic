import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../components/ui'
import { BookReaderView } from '../components/reader/BookReaderView'
import type { ReaderPageData } from '../components/reader/ReaderPage'
import { api } from '../api/client'
import type { CommunityBookDetail, Panel, ReaderChapter } from '../api/types'
import { errorMessage } from '../api/errors'
import { useAuth } from '../auth/useAuth'
import { getClientId } from '../lib/clientId'

// 判断一章是否有已渲染（有图）的分镜。
function hasRenderedPanels(panels: Panel[]): boolean {
  return panels.some((p) => p.status === 'done' && Boolean(p.imageUrl))
}

// 把公开书章节按 BookReader 同规则组装成翻页页：去掉封面章、只留有图的章。
function toChapterPages(chapters: ReaderChapter[]): ReaderPageData[] {
  return [...chapters]
    .filter((ch) => !ch.isCover)
    .sort((a, b) => a.order - b.order)
    .filter((ch) => hasRenderedPanels(ch.panels))
    .map((ch) => ({
      kind: 'chapter' as const,
      chapterTitle: ch.title,
      panels: ch.panels,
      summary: ch.summary ?? '',
    }))
}

// 社区公开阅读器容器：匿名/登录都能翻页读一本公开书。
// 挂载时拉详情 + 记一次独立浏览（useRef 守卫防 StrictMode 双触发），底部带点赞栏。
export default function CommunityReader() {
  const { id } = useParams<{ id: string }>()
  const bookId = Number(id)

  const [detail, setDetail] = useState<CommunityBookDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // 已记过 view 的书 id：同一 id 只记一次（StrictMode 会双执行 effect）。
  const viewedRef = useRef<number | null>(null)

  useEffect(() => {
    if (!Number.isFinite(bookId)) return
    let alive = true
    ;(async () => {
      setLoading(true)
      setError('')
      try {
        const data = await api.getCommunityBook(bookId)
        if (!alive) return
        setDetail(data)
        // 拉到详情后记一次独立浏览，失败静默（不阻断阅读）。
        if (viewedRef.current !== bookId) {
          viewedRef.current = bookId
          void api.recordView(bookId, getClientId()).catch(() => {})
        }
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
    if (!detail) return null
    return {
      kind: 'cover',
      bookId: detail.id,
      title: detail.title,
      coverUrl: detail.coverUrl,
      summary: detail.summary ?? '',
    }
  }, [detail])

  const chapterPages = useMemo<ReaderPageData[]>(
    () => (detail ? toChapterPages(detail.chapters) : []),
    [detail],
  )

  return (
    <BookReaderView
      loading={loading}
      error={error}
      title={detail?.title ?? ''}
      coverPage={coverPage}
      chapterPages={chapterPages}
      backTo="/community"
      backLabel="回到社区"
      headerRight={
        <Link to="/community">
          <Button variant="ghost" className="text-sm">
            ← 社区
          </Button>
        </Link>
      }
      footer={detail ? <LikeBar detail={detail} /> : null}
    />
  )
}

interface LikeBarProps {
  detail: CommunityBookDetail
}

// 点赞栏：显示 ❤ 数；登录用户点击切换点赞，未登录点击去登录。
function LikeBar({ detail }: LikeBarProps) {
  const { isAuthed } = useAuth()
  const navigate = useNavigate()

  const [liked, setLiked] = useState(detail.liked)
  const [likeCount, setLikeCount] = useState(detail.likeCount)
  const [pending, setPending] = useState(false)

  const onToggle = useCallback(async () => {
    if (!isAuthed) {
      navigate('/login')
      return
    }
    if (pending) return
    setPending(true)
    try {
      const result = liked ? await api.unlikeBook(detail.id) : await api.likeBook(detail.id)
      setLiked(result.liked)
      setLikeCount(result.likeCount)
    } catch {
      // 点赞失败静默：保留当前展示，用户可重试。
    } finally {
      setPending(false)
    }
  }, [isAuthed, navigate, pending, liked, detail.id])

  return (
    <div className="flex justify-center">
      <button
        type="button"
        onClick={onToggle}
        disabled={pending}
        aria-pressed={liked}
        aria-label={liked ? '取消点赞' : '点赞'}
        className="flex items-center gap-2 rounded-full bg-white px-5 py-2.5 font-display font-bold text-ink shadow-soft-sm transition-all hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-60"
      >
        <span aria-hidden className={liked ? 'text-coral' : 'text-ink-soft'}>
          ❤
        </span>
        <span>{likeCount}</span>
      </button>
    </div>
  )
}
