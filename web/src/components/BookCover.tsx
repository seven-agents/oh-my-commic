import { mediaUrl } from '../api/media'

type BookCoverProps = {
  id: number
  title: string
  coverUrl?: string
}

// 按书 id 变化的柔和渐变主题（cream/sky/meadow/peach/sun 系）。
const PALETTES = [
  { from: 'from-sky/50', to: 'to-meadow/40', emoji: '🌳' },
  { from: 'from-peach/50', to: 'to-sun/40', emoji: '🎈' },
  { from: 'from-meadow/50', to: 'to-sky/40', emoji: '🐰' },
  { from: 'from-sun/50', to: 'to-peach/40', emoji: '🌻' },
  { from: 'from-sky/40', to: 'to-peach/40', emoji: '🐳' },
  { from: 'from-coral/40', to: 'to-sun/40', emoji: '🦊' },
]

// 生成的漫画书封面：有图用图，无图用柔和渐变 + 标题 + 可爱表情。
export function BookCover({ id, title, coverUrl }: BookCoverProps) {
  const src = mediaUrl(coverUrl)
  const palette = PALETTES[((id % PALETTES.length) + PALETTES.length) % PALETTES.length]

  if (src) {
    return (
      <div className="aspect-[3/4] w-full overflow-hidden rounded-3xl bg-cream">
        <img src={src} alt={title} className="h-full w-full object-cover" loading="lazy" />
      </div>
    )
  }

  return (
    <div
      className={`flex aspect-[3/4] w-full flex-col items-center justify-center gap-3 rounded-3xl bg-gradient-to-br ${palette.from} ${palette.to} p-5 text-center`}
    >
      <span className="text-5xl" aria-hidden>
        {palette.emoji}
      </span>
      <span className="line-clamp-3 font-display text-lg font-bold text-ink">{title}</span>
    </div>
  )
}
