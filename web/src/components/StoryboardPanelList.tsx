import { useState } from 'react'
import { StoryboardPanelCard } from './StoryboardPanelCard'
import { api } from '../api/client'
import type { Panel } from '../api/types'
import { errorMessage } from '../api/errors'
import type { AssetIndex } from './chapter/assetLookup'

type StoryboardPanelListProps = {
  panels: Panel[]
  index: AssetIndex
  onPanelsChange: (panels: Panel[]) => void
}

// Stage ① 右侧：实时结构化分镜列表，内联编辑后 PUT 持久化。
export function StoryboardPanelList({ panels, index, onPanelsChange }: StoryboardPanelListProps) {
  const [savingId, setSavingId] = useState<number | null>(null)
  const [error, setError] = useState('')

  const savePanel = async (panel: Panel, patch: Partial<Panel>) => {
    setSavingId(panel.id)
    setError('')
    const merged = { ...panel, ...patch }
    const body = {
      caption: merged.caption,
      characterIds: merged.characterIds,
      sceneId: merged.sceneId,
      imagePrompt: merged.imagePrompt,
      location: merged.location,
      event: merged.event,
      charExpressions: merged.charExpressions,
    }
    try {
      const saved = await api.put<Panel>(`/api/panels/${panel.id}`, body)
      onPanelsChange(panels.map((p) => (p.id === saved.id ? saved : p)))
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSavingId(null)
    }
  }

  if (panels.length === 0) {
    return (
      <div className="flex h-full min-h-[280px] flex-col items-center justify-center gap-3 rounded-4xl bg-cream/60 p-6 text-center">
        <span className="text-4xl" aria-hidden>
          🎬
        </span>
        <p className="font-display font-semibold text-ink-soft">分镜会出现在这里</p>
        <p className="text-sm text-ink-soft/70">和左边的小助手聊聊你的故事吧～</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between px-1">
        <h3 className="font-display text-lg font-bold text-ink">故事分镜</h3>
        <span className="text-xs text-ink-soft/70">每格最多 3 个角色哦</span>
      </div>

      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-2.5 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}

      {panels.map((panel) => (
        <StoryboardPanelCard
          key={panel.id}
          panel={panel}
          index={index}
          saving={savingId === panel.id}
          onPatch={(patch) => savePanel(panel, patch)}
        />
      ))}
    </div>
  )
}
