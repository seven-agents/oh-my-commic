import { useState } from 'react'
import { Button, LoadingClouds } from './ui'
import { mediaUrl } from '../api/media'
import type { Panel } from '../api/types'
import { type AssetIndex, resolveActors, resolveScene } from './chapter/assetLookup'

type PanelCardProps = {
  panel: Panel
  index: AssetIndex
  onSaveCaption: (caption: string) => void
  onRemoveActor: (actorId: number) => void
  onRender: () => void
}

// Stage ② 单张分镜卡片：可编辑字幕、增删角色 chip、逐格生图。
export function PanelCard({ panel, index, onSaveCaption, onRemoveActor, onRender }: PanelCardProps) {
  const [caption, setCaption] = useState(panel.caption)
  const actors = resolveActors(panel.characterIds, index)
  const scene = resolveScene(panel.sceneId, index)

  const commitCaption = () => {
    const trimmed = caption.trim()
    if (trimmed !== panel.caption) onSaveCaption(trimmed)
  }

  return (
    <div className="flex flex-col gap-3 rounded-4xl bg-white p-4 shadow-soft">
      <div className="flex items-center justify-between">
        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-cream font-display font-bold text-ink-soft shadow-soft-sm">
          {panel.order + 1}
        </span>
        <span className="font-display text-sm font-semibold text-ink-soft">第 {panel.order + 1} 格</span>
      </div>

      <PanelImage panel={panel} onRender={onRender} />

      <textarea
        value={caption}
        onChange={(e) => setCaption(e.target.value)}
        onBlur={commitCaption}
        rows={2}
        placeholder="这一格讲了什么…"
        className="field resize-none text-sm"
      />

      {(actors.length > 0 || scene) && (
        <div className="flex flex-wrap items-center gap-1.5">
          {actors.map((a) => (
            <span
              key={a.id}
              className="inline-flex items-center gap-1 rounded-full bg-sky/20 px-2.5 py-1 text-xs font-semibold text-sky-deep"
            >
              {a.emoji} {a.name}
              <button
                type="button"
                onClick={() => onRemoveActor(a.id)}
                className="ml-0.5 text-sky-deep/60 hover:text-coral"
                aria-label={`移除 ${a.name}`}
              >
                ✕
              </button>
            </span>
          ))}
          {scene && (
            <span className="inline-flex items-center gap-1 rounded-full bg-meadow/25 px-2.5 py-1 text-xs font-semibold text-meadow-deep">
              🏞️ {scene.name}
            </span>
          )}
        </div>
      )}
    </div>
  )
}

function PanelImage({ panel, onRender }: { panel: Panel; onRender: () => void }) {
  const src = mediaUrl(panel.imageUrl)

  if (panel.status === 'rendering') {
    return (
      <div className="flex aspect-[4/3] items-center justify-center rounded-3xl bg-cream/60">
        <LoadingClouds label="正在画…" />
      </div>
    )
  }

  if (panel.status === 'done' && src) {
    return (
      <div className="group relative overflow-hidden rounded-3xl">
        <img src={src} alt={panel.caption} className="aspect-[4/3] w-full object-cover" loading="lazy" />
        <button
          type="button"
          onClick={onRender}
          className="absolute bottom-2 right-2 rounded-full bg-white/90 px-3 py-1.5 text-xs font-bold text-ink shadow-soft-sm transition-all hover:bg-white"
        >
          🔄 重新生成
        </button>
      </div>
    )
  }

  if (panel.status === 'failed') {
    return (
      <div className="flex aspect-[4/3] flex-col items-center justify-center gap-2 rounded-3xl bg-coral/10 text-center">
        <span className="text-3xl" aria-hidden>
          🌧️
        </span>
        <p className="text-sm font-semibold text-coral">没画成，再试一次～</p>
        <Button onClick={onRender} className="text-sm">
          再试一次
        </Button>
      </div>
    )
  }

  // pending
  return (
    <div className="flex aspect-[4/3] flex-col items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink/10 bg-cream/40">
      <Button onClick={onRender} className="text-sm">
        🎨 生成这张图
      </Button>
    </div>
  )
}
