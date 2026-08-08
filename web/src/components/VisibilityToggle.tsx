import type { Book } from '../api/types'
import { useSubmitOnce } from '../hooks/useSubmitOnce'

type VisibilityToggleProps = {
  book: Book
  onToggle: (book: Book) => Promise<void>
}

// 书卡上的「公开/私密」开关：公开时展示点赞/浏览计数与「公开」徽标。
// 点击调用 onToggle（内部走 api.setVisibility），useSubmitOnce 防双击。
export function VisibilityToggle({ book, onToggle }: VisibilityToggleProps) {
  const { submit, submitting } = useSubmitOnce(() => onToggle(book))

  return (
    <div className="mt-2 flex items-center justify-between gap-2 px-1">
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation()
          void submit()
        }}
        disabled={submitting}
        aria-pressed={book.isPublic}
        className={`rounded-full px-3 py-1 text-xs font-semibold transition-colors disabled:opacity-60 ${
          book.isPublic
            ? 'bg-mint/20 text-mint hover:bg-mint/30'
            : 'bg-ink/5 text-ink-soft hover:bg-ink/10'
        }`}
      >
        {book.isPublic ? '🌍 公开中' : '🔒 设为公开'}
      </button>

      {book.isPublic && (
        <div className="flex items-center gap-2 text-xs text-ink-soft">
          <span className="rounded-full bg-mint/15 px-2 py-0.5 font-semibold text-mint">公开</span>
          <span>❤{book.likeCount}</span>
          <span>👁{book.viewCount}</span>
        </div>
      )}
    </div>
  )
}
