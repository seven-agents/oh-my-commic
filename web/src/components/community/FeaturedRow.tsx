import { Link } from 'react-router-dom'
import type { CommunityBook } from '../../api/types'
import { mediaUrl } from '../../api/media'

// 精选一排：放大卡，突出封面 + 标题 + 作者 + 点赞/浏览。
export function FeaturedRow({ books }: { books: CommunityBook[] }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      {books.map((b) => {
        const cover = mediaUrl(b.coverUrl)
        return (
          <Link
            key={b.id}
            to={`/community/books/${b.id}`}
            className="group flex flex-col overflow-hidden rounded-3xl bg-white shadow-soft-sm transition-transform hover:-translate-y-1"
          >
            <div className="flex aspect-[16/9] items-center justify-center bg-gradient-to-br from-sky/20 to-peach/20 text-4xl">
              {cover ? (
                <img src={cover} alt={b.title} className="h-full w-full object-cover" />
              ) : (
                <span aria-hidden>📖</span>
              )}
            </div>
            <div className="flex flex-col gap-1 p-4">
              <h3 className="truncate font-display text-lg font-extrabold text-ink">{b.title}</h3>
              <p className="truncate text-sm text-ink-soft">by {b.author.nickname}</p>
              <p className="text-sm text-ink-soft">❤ {b.likeCount}　👁 {b.viewCount}</p>
            </div>
          </Link>
        )
      })}
    </div>
  )
}
