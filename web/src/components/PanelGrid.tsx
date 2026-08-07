import { useMemo, useState } from 'react'
import { Button, Card } from './ui'
import { PanelCard } from './PanelCard'
import { api } from '../api/client'
import type { Panel } from '../api/types'
import { errorMessage } from '../api/errors'
import { type AssetIndex, resolveActors, resolveScene } from './chapter/assetLookup'

type PanelGridProps = {
  panels: Panel[]
  index: AssetIndex
  onPanelsChange: (panels: Panel[]) => void
  onNext: () => void
}

// Stage ② 画分镜：出场概览 + 逐格生图 + 全部生成。
export function PanelGrid({ panels, index, onPanelsChange, onNext }: PanelGridProps) {
  const [error, setError] = useState('')
  const [bulk, setBulk] = useState(false)

  const replacePanel = (updated: Panel) =>
    onPanelsChange(panels.map((p) => (p.id === updated.id ? updated : p)))

  const setPanelStatus = (id: number, status: Panel['status']) =>
    onPanelsChange(panels.map((p) => (p.id === id ? { ...p, status } : p)))

  const savePanel = async (panel: Panel, patch: Partial<Panel>) => {
    setError('')
    const body = {
      caption: patch.caption ?? panel.caption,
      characterIds: patch.characterIds ?? panel.characterIds,
      sceneId: patch.sceneId ?? panel.sceneId,
      imagePrompt: patch.imagePrompt ?? panel.imagePrompt,
    }
    try {
      const saved = await api.put<Panel>(`/api/panels/${panel.id}`, body)
      replacePanel(saved)
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  const renderPanel = async (panel: Panel) => {
    setError('')
    setPanelStatus(panel.id, 'rendering')
    try {
      const done = await api.post<Panel>(`/api/panels/${panel.id}/render`)
      replacePanel(done)
    } catch (err) {
      setError(errorMessage(err))
      setPanelStatus(panel.id, 'failed')
    }
  }

  const renderAll = async () => {
    setBulk(true)
    setError('')
    // 逐格顺序生成，避免压垮上游；每次读最新状态。
    for (const p of panels) {
      if (p.status === 'done') continue
      setPanelStatus(p.id, 'rendering')
      try {
        const done = await api.post<Panel>(`/api/panels/${p.id}/render`)
        replacePanel(done)
      } catch (err) {
        setError(errorMessage(err))
        setPanelStatus(p.id, 'failed')
      }
    }
    setBulk(false)
  }

  const summary = useMemo(() => buildCastSummary(panels, index), [panels, index])
  const doneCount = panels.filter((p) => p.status === 'done').length
  const allDone = panels.length > 0 && doneCount === panels.length
  const anyPending = panels.some((p) => p.status !== 'done')

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
        <Button onClick={renderAll} loading={bulk} disabled={!anyPending} variant="ghost" className="text-sm">
          🎨 全部生成
        </Button>
      </div>

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
