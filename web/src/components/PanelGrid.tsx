import { useMemo, useState, type Dispatch, type SetStateAction } from 'react'
import { Button, Card } from './ui'
import { PanelCard } from './PanelCard'
import { api } from '../api/client'
import type { Panel } from '../api/types'
import { errorMessage } from '../api/errors'
import { type AssetIndex, resolveActors, resolveScene } from './chapter/assetLookup'
import { canRenderPanel } from './chapter/panelStage'

type PanelGridProps = {
  panels: Panel[]
  index: AssetIndex
  onPanelsChange: Dispatch<SetStateAction<Panel[]>>
  onNext: () => void
}

// Stage ② 画分镜：出场概览 + 逐格生图 + 全部生成。
export function PanelGrid({ panels, index, onPanelsChange, onNext }: PanelGridProps) {
  const [error, setError] = useState('')
  const [bulk, setBulk] = useState(false)

  // 函数式更新：每次都基于最新 state 打补丁，避免快照覆盖已生成分镜。
  const applyPanel = (id: number, patch: Partial<Panel>) =>
    onPanelsChange((prev) => prev.map((p) => (p.id === id ? { ...p, ...patch } : p)))

  const savePanel = async (panel: Panel, patch: Partial<Panel>) => {
    setError('')
    const merged = { ...panel, ...patch }
    const body = {
      content: merged.content,
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
      applyPanel(saved.id, saved)
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  const renderPanel = async (panel: Panel) => {
    // 未解析的格不出图（会画出没内容的通用图）。
    if (!canRenderPanel(panel)) return
    setError('')
    applyPanel(panel.id, { status: 'rendering' })
    try {
      const done = await api.post<Panel>(`/api/panels/${panel.id}/render`)
      applyPanel(done.id, done)
    } catch (err) {
      setError(errorMessage(err))
      applyPanel(panel.id, { status: 'failed' })
    }
  }

  const renderAll = async () => {
    setBulk(true)
    setError('')
    // 逐格顺序生成，避免压垮上游；每格用函数式更新叠加到最新 state。
    // 未解析的格跳过（无结构字段，出图无意义）。
    for (const p of panels) {
      if (p.status === 'done' || !canRenderPanel(p)) continue
      applyPanel(p.id, { status: 'rendering' })
      try {
        const done = await api.post<Panel>(`/api/panels/${p.id}/render`)
        applyPanel(done.id, done)
      } catch (err) {
        setError(errorMessage(err))
        applyPanel(p.id, { status: 'failed' })
      }
    }
    setBulk(false)
  }

  const summary = useMemo(() => buildCastSummary(panels, index), [panels, index])
  const doneCount = panels.filter((p) => p.status === 'done').length
  const allDone = panels.length > 0 && doneCount === panels.length
  // 可全部生成：存在已解析但未出图的格。
  const anyRenderable = panels.some((p) => p.status !== 'done' && canRenderPanel(p))
  const unprocessedCount = panels.filter((p) => !canRenderPanel(p)).length

  return (
    <div className="flex flex-col gap-6">
      <Card className="flex flex-col gap-3">
        <h3 className="font-display text-lg font-bold text-ink">AI 识别到出场：</h3>
        {summary.length === 0 ? (
          <p className="text-sm text-ink-soft/70">这一章还没识别到具体的角色或场景，先生成图看看吧～</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {summary.map((chip) => (
              <span
                key={chip.key}
                className={`inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-sm font-semibold ${chip.className}`}
              >
                {chip.emoji} {chip.name}
              </span>
            ))}
          </div>
        )}
      </Card>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="font-display text-ink-soft">
          已完成 <span className="font-bold text-ink">{doneCount}</span> / {panels.length} 格
        </p>
        <Button onClick={renderAll} loading={bulk} disabled={!anyRenderable} variant="ghost" className="text-sm">
          🎨 全部生成
        </Button>
      </div>

      {unprocessedCount > 0 && (
        <p className="rounded-2xl bg-peach/15 px-4 py-2.5 text-center text-sm font-semibold text-ink-soft">
          还有 {unprocessedCount} 格没解析，先回上一步点「解析这格」再出图哦
        </p>
      )}

      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}

      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
        {panels.map((panel) => (
          <PanelCard
            key={panel.id}
            panel={panel}
            index={index}
            canRender={canRenderPanel(panel)}
            onSaveCaption={(caption) => savePanel(panel, { caption })}
            onRemoveActor={(actorId) =>
              savePanel(panel, { characterIds: panel.characterIds.filter((id) => id !== actorId) })
            }
            onRender={() => renderPanel(panel)}
          />
        ))}
      </div>

      <div className="flex justify-center">
        <Button onClick={onNext} disabled={!allDone} className="text-base">
          下一步：拼成书 →
        </Button>
      </div>
      {!allDone && (
        <p className="text-center text-xs text-ink-soft/70">把每一格都画好，就能拼成书啦～</p>
      )}
    </div>
  )
}

type CastChip = { key: string; name: string; emoji: string; className: string }

function buildCastSummary(panels: Panel[], index: AssetIndex): CastChip[] {
  const chips = new Map<string, CastChip>()
  for (const p of panels) {
    for (const a of resolveActors(p.characterIds, index)) {
      chips.set(`c${a.id}`, {
        key: `c${a.id}`,
        name: a.name,
        emoji: a.emoji,
        className: 'bg-sky/20 text-sky-deep',
      })
    }
    const scene = resolveScene(p.sceneId, index)
    if (scene) {
      chips.set(`s${scene.id}`, {
        key: `s${scene.id}`,
        name: scene.name,
        emoji: '🏞️',
        className: 'bg-meadow/25 text-meadow-deep',
      })
    }
  }
  return [...chips.values()]
}
