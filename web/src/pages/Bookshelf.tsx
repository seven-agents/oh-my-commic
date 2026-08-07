import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { BookCover } from '../components/BookCover'
import { Button, EmptyState, Input, LoadingClouds, Modal } from '../components/ui'
import { api } from '../api/client'
import type { Book } from '../api/types'
import { errorMessage } from '../api/errors'

export default function Bookshelf() {
  const navigate = useNavigate()
  const [books, setBooks] = useState<Book[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [createOpen, setCreateOpen] = useState(false)

  useEffect(() => {
    let alive = true
    ;(async () => {
      try {
        const data = await api.get<Book[]>('/api/books')
        if (alive) setBooks(data ?? [])
      } catch (err) {
        if (alive) setLoadError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  const onCreated = (book: Book) => {
    setBooks((prev) => [book, ...prev])
    setCreateOpen(false)
    navigate(`/books/${book.id}`)
  }

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader />

      <main className="mx-auto max-w-6xl px-6 py-8">
        <div className="mb-6">
          <h1 className="font-display text-3xl font-extrabold text-ink">我的书架 📚</h1>
          <p className="mt-1 text-ink-soft">摆满你亲手画的漫画书～</p>
        </div>

        {loading ? (
          <LoadingClouds label="正在打开书架…" />
        ) : loadError ? (
          <EmptyState emoji="🌧️" title="书架没打开" description={loadError} />
        ) : books.length === 0 ? (
          <EmptyState
            emoji="📚"
            title="还没有漫画书哦"
            description="来创作第一本吧！"
            action={<Button onClick={() => setCreateOpen(true)}>＋ 创作新书</Button>}
          />
        ) : (
          <div className="grid grid-cols-2 gap-5 sm:grid-cols-3 lg:grid-cols-4">
            <CreateCard onClick={() => setCreateOpen(true)} />
            {books.map((book) => (
              <button
                key={book.id}
                type="button"
                onClick={() => navigate(`/books/${book.id}`)}
                className="group animate-pop-in text-left"
              >
                <div className="transition-transform duration-200 group-hover:-translate-y-1">
                  <BookCover id={book.id} title={book.title} coverUrl={book.coverUrl} />
                </div>
                <p className="mt-2 truncate px-1 font-display font-semibold text-ink">{book.title}</p>
              </button>
            ))}
          </div>
        )}
      </main>

      <CreateBookModal open={createOpen} onClose={() => setCreateOpen(false)} onCreated={onCreated} />
    </div>
  )
}

function CreateCard({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="group flex aspect-[3/4] w-full flex-col items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink/15 bg-white/50 text-center transition-all hover:-translate-y-1 hover:border-coral/40 hover:bg-white"
    >
      <span className="text-5xl transition-transform group-hover:scale-110" aria-hidden>
        ＋
      </span>
      <span className="font-display font-semibold text-ink-soft group-hover:text-ink">创作新书</span>
    </button>
  )
}

function CreateBookModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (book: Book) => void
}) {
  const [title, setTitle] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const onSubmit = async () => {
    const trimmed = title.trim()
    if (!trimmed) {
      setError('先给新书起个名字吧～')
      return
    }
    setSubmitting(true)
    setError('')
    try {
      const book = await api.post<Book>('/api/books', { title: trimmed, style: 'ghibli' })
      setTitle('')
      onCreated(book)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="创作一本新书 ✨">
      <div className="flex flex-col gap-4">
        <Input
          id="new-book-title"
          label="书名"
          placeholder="例如：小狐狸的森林冒险"
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
            开始创作 🖍️
          </Button>
        </div>
      </div>
    </Modal>
  )
}
