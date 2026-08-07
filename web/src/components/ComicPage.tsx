import { mediaUrl } from '../api/media'
import { EmptyState } from './ui'
import type { Panel } from '../api/types'

type ComicPageProps = {
  panels: Panel[]
}

// 把分镜图拼成一页漫画（2 列漫画格 + 字幕条）。编辑器与阅读页共用。
export function ComicPage({ panels }: ComicPageProps) {
  const cells = panels.filter((p) => p.status === 'done' && p.imageUrl)

  if (cells.length === 0) {
    return (
      <EmptyState
        emoji="🎨"
        title="还没有画好的分镜"
        description="先回到上一步，把每一格都画出来吧～"
      />
    )
  }

  return (
    <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
      {cells.map((panel) => (
        <ComicCell key={panel.id} panel={panel} />
      ))}
    </div>
  )
}

function ComicCell({ panel }: { panel: Panel }) {
  return (
    <figure className="overflow-hidden rounded-4xl border-4 border-white bg-white shadow-soft-lg">
      <img
        src={mediaUrl(panel.imageUrl)}
        alt={panel.caption}
        className="aspect-[4/3] w-full object-cover"
        loading="lazy"
      />
      {panel.caption && (
        <figcaption className="bg-parchment px-4 py-3 text-center font-body text-sm leading-relaxed text-ink">
          {panel.caption}
        </figcaption>
      )}
    </figure>
  )
}
