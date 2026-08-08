import { BookCover } from './BookCover'
import { Button } from './ui'
import { VisibilityToggle } from './VisibilityToggle'
import type { Book } from '../api/types'

type BookCardProps = {
  book: Book
  onRead: (id: number) => void
  onEdit: (id: number) => void
  onDelete: (book: Book) => void
  onToggleVisibility: (book: Book) => Promise<void>
}

// 书架上的一本书：封面点开即「阅读」，下方两枚按钮「阅读 / 编辑」，右上角删除小按钮。
export function BookCard({ book, onRead, onEdit, onDelete, onToggleVisibility }: BookCardProps) {
  return (
    <div className="group relative animate-pop-in">
      <button
        type="button"
        onClick={() => onRead(book.id)}
        className="w-full text-left"
      >
        <div className="transition-transform duration-200 group-hover:-translate-y-1">
          <BookCover id={book.id} title={book.title} coverUrl={book.coverUrl} />
        </div>
        <p className="mt-2 truncate px-1 font-display font-semibold text-ink">{book.title}</p>
      </button>

      <div className="mt-2 flex gap-2 px-1">
        <Button
          className="flex-1 px-3 py-2 text-sm"
          onClick={(e) => {
            e.stopPropagation()
            onRead(book.id)
          }}
        >
          📖 阅读
        </Button>
        <Button
          variant="ghost"
          className="flex-1 px-3 py-2 text-sm"
          onClick={(e) => {
            e.stopPropagation()
            onEdit(book.id)
          }}
        >
          ✏️ 编辑
        </Button>
      </div>

      <VisibilityToggle book={book} onToggle={onToggleVisibility} />

      <button
        type="button"
        aria-label={`删除${book.title}`}
        onClick={(e) => {
          e.stopPropagation()
          onDelete(book)
        }}
        className="absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full bg-white/90 text-base text-coral shadow-soft-sm transition-all hover:bg-coral hover:text-white focus:opacity-100 group-hover:opacity-100 sm:opacity-0"
      >
        <span aria-hidden>✕</span>
      </button>
    </div>
  )
}
