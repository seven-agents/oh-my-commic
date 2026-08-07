import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Input, Modal } from './ui'
import { api } from '../api/client'
import type { Chapter, ChapterStatus } from '../api/types'
import { errorMessage } from '../api/errors'

type ChapterListProps = {
  bookId: string
  chapters: Chapter[]
  onCreated: (chapter: Chapter) => void
}

const STATUS_META: Record<ChapterStatus, { label: string; className: string }> = {
  draft: { label: '草稿', className: 'bg-ink/10 text-ink-soft' },
  storyboarding: { label: '分镜中', className: 'bg-sky/25 text-sky-deep' },
  rendering: { label: '出图中', className: 'bg-sun/30 text-peach' },
  done: { label: '完成', className: 'bg-meadow/30 text-meadow-deep' },
}

export function ChapterList({ bookId, chapters, onCreated }: ChapterListProps) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-2xl font-extrabold text-ink">章节 📖</h2>
        <Button onClick={() => setOpen(true)} className="text-sm">
          ＋ 新建章节
        </Button>
      </div>

      {chapters.length === 0 ? (
        <p className="rounded-3xl bg-cream/70 px-5 py-8 text-center text-ink-soft">
          还没有章节，点上面的按钮开始第一章吧～
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {chapters.map((ch) => {
            const meta = STATUS_META[ch.status]
            return (
              <li key={ch.id}>
                <button
                  type="button"
                  onClick={() => navigate(`/chapters/${ch.id}`)}
                  className="flex w-full items-center gap-3 rounded-3xl bg-cream/60 px-5 py-4 text-left transition-all hover:-translate-y-0.5 hover:bg-white hover:shadow-soft-sm"
                >
                  <span className="flex h-9 w-9 flex-none items-center justify-center rounded-full bg-white font-display font-bold text-ink-soft shadow-soft-sm">
                    {ch.order + 1}
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

  const onSubmit = async () => {
    const trimmed = title.trim()
    if (!trimmed) {
      setError('给这一章起个名字吧～')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      const ch = await api.post<Chapter>(`/api/books/${bookId}/chapters`, { title: trimmed })
      setTitle('')
      onCreated(ch)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
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
