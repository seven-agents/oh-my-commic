import { BookCover } from './BookCover'
import type { Book } from '../api/types'

type BookCardProps = {
  book: Book
  onOpen: (id: number) => void
  onDelete: (book: Book) => void
}

// 书架上的一本书：点击进入，右上角有删除小按钮（悬停显示 / 触摸可点）。
export function BookCard({ book, onOpen, onDelete }: BookCardProps) {
  return (
    <div className="group relative animate-pop-in">
      <button
        type="button"
        onClick={() => onOpen(book.id)}
        className="w-full text-left"
      >
        <div className="transition-transform duration-200 group-hover:-translate-y-1">
          <BookCover id={book.id} title={book.title} coverUrl={book.coverUrl} />
        </div>
        <p className="mt-2 truncate px-1 font-display font-semibold text-ink">{book.title}</p>
      </button>
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
