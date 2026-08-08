import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Input, Modal } from './ui'
import { BookCover } from './BookCover'
import { api } from '../api/client'
import { cnNumeral } from '../api/cnNumeral'
import type { Book, Chapter, ChapterStatus } from '../api/types'
import { errorMessage } from '../api/errors'

type ChapterListProps = {
  book: Book
  chapters: Chapter[]
  onCreated: (chapter: Chapter) => void
}

const STATUS_META: Record<ChapterStatus, { label: string; className: string }> = {
  draft: { label: '草稿', className: 'bg-ink/10 text-ink-soft' },
  storyboarding: { label: '分镜中', className: 'bg-sky/25 text-sky-deep' },
  rendering: { label: '出图中', className: 'bg-sun/30 text-peach' },
  done: { label: '完成', className: 'bg-meadow/30 text-meadow-deep' },
}

export function ChapterList({ book, chapters, onCreated }: ChapterListProps) {
  const navigate = useNavigate()
  const bookId = String(book.id)
  const [open, setOpen] = useState(false)

  const cover = chapters.find((c) => c.isCover)
  const regularChapters = chapters
    .filter((c) => !c.isCover)
    .sort((a, b) => a.order - b.order)

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-2xl font-extrabold text-ink">章节 📖</h2>
        <Button onClick={() => setOpen(true)} className="text-sm">
          ＋ 新建章节
        </Button>
      </div>

      <CoverCard book={book} cover={cover} onCreated={onCreated} />

      {regularChapters.length === 0 ? (
        <p className="rounded-3xl bg-cream/70 px-5 py-8 text-center text-ink-soft">
          还没有章节，点上面的按钮开始第一章吧～
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {regularChapters.map((ch) => {
            const meta = STATUS_META[ch.status]
            return (
              <li key={ch.id}>
                <button
                  type="button"
                  onClick={() => navigate(`/chapters/${ch.id}`)}
                  className="flex w-full items-center gap-3 rounded-3xl bg-cream/60 px-5 py-4 text-left transition-all hover:-translate-y-0.5 hover:bg-white hover:shadow-soft-sm"
                >
                  <span className="flex h-9 w-9 flex-none items-center justify-center rounded-full bg-white font-display text-xs font-bold text-ink-soft shadow-soft-sm">
                    第{cnNumeral(ch.order)}章
                  </span>
                  <span className="min-w-0 flex-1 truncate font-display font-semibold text-ink">
                    {ch.title}
                  </span>
                  <span className={`flex-none rounded-full px-3 py-1 text-xs font-bold ${meta.className}`}>
                    {meta.label}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}

      <NewChapterModal
        open={open}
        bookId={bookId}
        onClose={() => setOpen(false)}
        onCreated={(ch) => {
          setOpen(false)
          onCreated(ch)
          navigate(`/chapters/${ch.id}`)
        }}
      />
    </Card>
  )
}

// 封面卡：有封面章 → 缩略图入口；无 → 「制作封面」按需创建后跳转。
function CoverCard({
  book,
  cover,
  onCreated,
}: {
  book: Book
  cover: Chapter | undefined
  onCreated: (chapter: Chapter) => void
}) {
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)
  const creatingRef = useRef(false)
  const [error, setError] = useState('')

  if (cover) {
    return (
      <button
        type="button"
        onClick={() => navigate(`/chapters/${cover.id}`)}
        className="flex w-full items-center gap-4 rounded-3xl bg-cream/60 px-5 py-4 text-left transition-all hover:-translate-y-0.5 hover:bg-white hover:shadow-soft-sm"
      >
        <span className="w-16 flex-none">
          <BookCover id={book.id} title="封面" coverUrl={book.coverUrl} />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block font-display font-semibold text-ink">封面</span>
          <span className="block text-xs text-ink-soft">这本书的封面 🎨</span>
        </span>
        <span className="flex-none rounded-full bg-sun/30 px-3 py-1 text-xs font-bold text-peach">
          封面
        </span>
      </button>
    )
  }

  const createCover = async () => {
    if (creatingRef.current) return
    creatingRef.current = true
    setCreating(true)
    setError('')
    try {
      const c = await api.post<Chapter>(`/api/books/${book.id}/cover-chapter`, {})
      onCreated(c)
      navigate(`/chapters/${c.id}`)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      creatingRef.current = false
      setCreating(false)
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <button
        type="button"
        onClick={createCover}
        disabled={creating}
        className="flex w-full items-center gap-3 rounded-3xl border-2 border-dashed border-ink/10 bg-cream/40 px-5 py-4 text-left transition-all hover:-translate-y-0.5 hover:bg-white hover:shadow-soft-sm disabled:cursor-not-allowed disabled:opacity-60"
      >
        <span className="flex h-9 w-9 flex-none items-center justify-center rounded-full bg-white font-display text-lg font-bold text-peach shadow-soft-sm">
          ＋
        </span>
        <span className="min-w-0 flex-1 font-display font-semibold text-ink">
          {creating ? '正在准备封面…' : '制作封面'}
        </span>
        <span className="flex-none text-xl" aria-hidden>
          🎨
        </span>
      </button>
      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-2 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}
    </div>
  )
}

function NewChapterModal({
  open,
  bookId,
  onClose,
  onCreated,
}: {
  open: boolean
  bookId: string
  onClose: () => void
  onCreated: (chapter: Chapter) => void
}) {
  const [title, setTitle] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  // 同步防重入：setSubmitting 是异步 state，快速双击 / 回车长按会在 re-render
  // 前重复进入并发两次 POST（造成重复章节）。用 ref 立即拦住。
  const submittingRef = useRef(false)

  const onSubmit = async () => {
    if (submittingRef.current) return
    const trimmed = title.trim()
    if (!trimmed) {
      setError('给这一章起个名字吧～')
      return
    }
    submittingRef.current = true
    setSubmitting(true)
    setError('')
    try {
      const ch = await api.post<Chapter>(`/api/books/${bookId}/chapters`, { title: trimmed })
      setTitle('')
      onCreated(ch)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      submittingRef.current = false
      setSubmitting(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="新建一章 ✍️">
      <div className="flex flex-col gap-4">
        <Input
          id="new-chapter-title"
          label="章节名"
          placeholder="例如：第一章 · 出发去森林"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onSubmit()
          }}
          autoFocus
        />
        {error && (
          <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
            {error}
          </p>
        )}
        <div className="mt-1 flex gap-3">
          <Button variant="ghost" onClick={onClose} className="flex-1">
            再想想
          </Button>
          <Button onClick={onSubmit} loading={submitting} className="flex-1">
            创建 ✨
          </Button>
        </div>
      </div>
    </Modal>
  )
}
