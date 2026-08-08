import { useRef, useState } from 'react'
import { Button } from './ui'
import { StoryboardPanelCard } from './StoryboardPanelCard'
import { api } from '../api/client'
import type { Panel } from '../api/types'
import { errorMessage } from '../api/errors'
import type { AssetIndex } from './chapter/assetLookup'
import { isPanelProcessed } from './chapter/panelStage'

type StoryboardPanelListProps = {
  panels: Panel[]
  index: AssetIndex
  onPanelsChange: (panels: Panel[]) => void
}

// PUT 时保留全部可编辑字段（含 content + imagePrompt）。
function panelBody(panel: Panel) {
  return {
    content: panel.content,
    caption: panel.caption,
    characterIds: panel.characterIds,
    sceneId: panel.sceneId,
    imagePrompt: panel.imagePrompt,
    location: panel.location,
    event: panel.event,
    charExpressions: panel.charExpressions,
  }
}

// Stage ① 右侧：逐格「基本内容」+「解析这格」拆出结构字段；内联编辑后 PUT 持久化。
export function StoryboardPanelList({ panels, index, onPanelsChange }: StoryboardPanelListProps) {
  const [savingId, setSavingId] = useState<number | null>(null)
  const [processingId, setProcessingId] = useState<number | null>(null)
  const [bulkProcessing, setBulkProcessing] = useState(false)
  const [error, setError] = useState('')
  // ref 防重入：解析/全部解析进行中时忽略重复触发。
  const busyRef = useRef(false)

  const replacePanel = (saved: Panel) =>
    onPanelsChange(panels.map((p) => (p.id === saved.id ? saved : p)))

  const savePanel = async (panel: Panel, patch: Partial<Panel>) => {
    setSavingId(panel.id)
    setError('')
    try {
      const saved = await api.put<Panel>(`/api/v1/panels/${panel.id}`, panelBody({ ...panel, ...patch }))
      replacePanel(saved)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSavingId(null)
    }
  }

  const processPanel = async (panel: Panel) => {
    if (busyRef.current) return
    busyRef.current = true
    setProcessingId(panel.id)
    setError('')
    try {
      const done = await api.post<Panel>(`/api/v1/panels/${panel.id}/process`)
      replacePanel(done)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setProcessingId(null)
      busyRef.current = false
    }
  }

  const processAll = async () => {
    if (busyRef.current) return
    busyRef.current = true
    setBulkProcessing(true)
    setError('')
    // 顺序解析所有未解析的格，逐个 await，逐个回填到最新列表。
    let current = panels
    for (const p of panels) {
      if (isPanelProcessed(p)) continue
      setProcessingId(p.id)
      try {
        const done = await api.post<Panel>(`/api/v1/panels/${p.id}/process`)
        current = current.map((x) => (x.id === done.id ? done : x))
        onPanelsChange(current)
      } catch (err) {
        setError(errorMessage(err))
        break
      }
    }
    setProcessingId(null)
    setBulkProcessing(false)
    busyRef.current = false
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

  const unprocessedCount = panels.filter((p) => !isPanelProcessed(p)).length

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2 px-1">
        <h3 className="font-display text-lg font-bold text-ink">故事分镜</h3>
        {unprocessedCount > 0 && (
          <Button
            onClick={processAll}
            loading={bulkProcessing}
            variant="ghost"
            className="text-sm"
          >
            ✨ 全部解析（{unprocessedCount}）
          </Button>
        )}
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
          processing={processingId === panel.id}
          onPatch={(patch) => savePanel(panel, patch)}
          onProcess={() => processPanel(panel)}
        />
      ))}
    </div>
  )
}
