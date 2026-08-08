import { useState } from 'react'
import { Button, LoadingClouds } from './ui'
import { StoryboardPanelCard } from './StoryboardPanelCard'
import { mediaUrl } from '../api/media'
import { cnNumeral } from '../api/cnNumeral'
import type { Panel } from '../api/types'
import type { AssetIndex } from './chapter/assetLookup'

type PanelCardProps = {
  panel: Panel
  index: AssetIndex
  saving: boolean
  processing: boolean
  // 是否已解析（结构字段就绪）；未解析时禁止出图。
  canRender: boolean
  onPatch: (patch: Partial<Panel>) => void
  onProcess: () => void
  onRender: () => void
}

// 出图阶段单卡：默认只显示 头部(第N格+摘要+编辑按钮) + 图片/出图，卡片高度统一、
// 左右两列对齐；编辑区(复用 StoryboardPanelCard 的可编辑输入/输出+重新解析)默认收起，
// 点「编辑」按需展开。
export function PanelCard({
  panel,
  index,
  saving,
  processing,
  canRender,
  onPatch,
  onProcess,
  onRender,
}: PanelCardProps) {
  const [editing, setEditing] = useState(false)
  const summary = panel.caption.trim() || (canRender ? '已解析，待出图' : '还没解析')

  return (
    <div className="flex flex-col gap-3 rounded-3xl bg-white/60 p-3 shadow-soft-sm">
      <div className="flex items-center gap-2">
        <span className="flex-none rounded-full bg-white px-2.5 py-1 font-display text-xs font-bold text-ink-soft shadow-soft-sm">
          第{cnNumeral(panel.order + 1)}格
        </span>
        <span className="min-w-0 flex-1 truncate text-sm text-ink-soft">{summary}</span>
        <button
          type="button"
          onClick={() => setEditing((v) => !v)}
          className="flex-none rounded-full bg-cream px-3 py-1 text-xs font-semibold text-ink transition-colors hover:bg-sky/20"
        >
          {editing ? '收起 ▲' : '✏️ 编辑'}
        </button>
      </div>

      <PanelRenderFooter panel={panel} canRender={canRender} onRender={onRender} />

      {editing && (
        <StoryboardPanelCard
          panel={panel}
          index={index}
          saving={saving}
          processing={processing}
          onPatch={onPatch}
          onProcess={onProcess}
        />
      )}
    </div>
  )
}

// 出图脚注：图片预览（done/rendering/failed）+「生成这张图」按钮（未解析禁用并提示）。
function PanelRenderFooter({
  panel,
  canRender,
  onRender,
}: {
  panel: Panel
  canRender: boolean
  onRender: () => void
}) {
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

  // pending：未解析的格禁止出图，给出提示。
  if (!canRender) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink/10 bg-cream/40 px-4 py-5 text-center">
        <span className="text-2xl" aria-hidden>
          ✨
        </span>
        <p className="text-sm font-semibold text-ink-soft">先点「重新解析」再生成图哦</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink/10 bg-cream/40 py-5">
      <Button onClick={onRender} className="text-sm">
        🎨 生成这张图
      </Button>
    </div>
  )
}
