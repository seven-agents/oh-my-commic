import { Link } from 'react-router-dom'
import { mediaUrl } from '../api/media'
import type { CommunityBook } from '../api/types'

interface CommunityCardProps {
  book: CommunityBook
}

// 社区 feed 里的一本公开漫画：封面 + 标题 + 概述 + 作者行 + 点赞/浏览。整卡可点进阅读页。
export function CommunityCard({ book }: CommunityCardProps) {
  const cover = mediaUrl(book.coverUrl)
  const { nickname, avatarUrl } = book.author
  const avatar = mediaUrl(avatarUrl)
  const initial = nickname.trim()[0] ?? '🙂'

  return (
    <Link
      to={`/community/books/${book.id}`}
      className="group flex animate-pop-in flex-col overflow-hidden rounded-3xl bg-white shadow-soft-sm transition-transform duration-200 hover:-translate-y-1"
    >
      <div className="aspect-[3/4] w-full overflow-hidden bg-cream">
        {cover ? (
          <img
            src={cover}
            alt={book.title}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-sky/40 to-peach/40 text-5xl">
            <span aria-hidden>📖</span>
          </div>
        )}
      </div>

      <div className="flex flex-1 flex-col gap-2 p-4">
        <p className="truncate font-display font-bold text-ink">{book.title}</p>
        {book.summary && (
          <p className="line-clamp-2 text-sm text-ink-soft">{book.summary}</p>
        )}

        <div className="mt-auto flex items-center justify-between pt-1">
          <span className="flex min-w-0 items-center gap-2">
            {avatar ? (
              <img
                src={avatar}
                alt={nickname}
                className="h-6 w-6 shrink-0 rounded-full object-cover"
              />
            ) : (
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gradient-to-b from-sun to-peach text-xs font-bold text-white">
                {initial}
              </span>
            )}
            <span className="truncate text-sm font-semibold text-ink-soft">{nickname}</span>
          </span>

          <span className="flex shrink-0 items-center gap-2 text-sm text-ink-soft">
            <span>❤ {book.likeCount}</span>
            <span>👁 {book.viewCount}</span>
          </span>
        </div>
      </div>
    </Link>
  )
}
