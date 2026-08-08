import { useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react'
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

// PUT 时保留全部可编辑字段（含 content + imagePrompt），避免丢字段。
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

// Stage ② 逐格出图：出场概览 + 可编辑输入/输出 + 重新解析 + 逐格/全部生成。
export function PanelGrid({ panels, index, onPanelsChange, onNext }: PanelGridProps) {
  const [error, setError] = useState('')
  const [bulk, setBulk] = useState(false)
  const [savingId, setSavingId] = useState<number | null>(null)
  const [processingId, setProcessingId] = useState<number | null>(null)
  // ref 防重入：全部生成 / 重新解析进行中时忽略重复触发。
  const bulkRef = useRef(false)
  const processRef = useRef(false)

  // 函数式更新：每次都基于最新 state 打补丁，避免快照覆盖已生成分镜。
  const applyPanel = (id: number, patch: Partial<Panel>) =>
    onPanelsChange((prev) => prev.map((p) => (p.id === id ? { ...p, ...patch } : p)))

  const savePanel = async (panel: Panel, patch: Partial<Panel>) => {
    setSavingId(panel.id)
    setError('')
    try {
      const saved = await api.put<Panel>(`/api/panels/${panel.id}`, panelBody({ ...panel, ...patch }))
      applyPanel(saved.id, saved)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSavingId(null)
    }
  }

  // 重新解析这格：从当前 content 重新推导结构字段，覆盖回填。
  const processPanel = async (panel: Panel) => {
    if (processRef.current) return
    processRef.current = true
    setProcessingId(panel.id)
    setError('')
    try {
      const done = await api.post<Panel>(`/api/panels/${panel.id}/process`)
      applyPanel(done.id, done)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setProcessingId(null)
      processRef.current = false
    }
  }

  // 单格出图：立即置 rendering，成功回填、失败标 failed。
  const renderOne = async (panel: Panel) => {
    applyPanel(panel.id, { status: 'rendering' })
    try {
      const done = await api.post<Panel>(`/api/panels/${panel.id}/render`)
      applyPanel(done.id, done)
    } catch (err) {
      setError(errorMessage(err))
      applyPanel(panel.id, { status: 'failed' })
    }
  }

  const renderPanel = async (panel: Panel) => {
    // 未解析的格不出图（会画出没内容的通用图）。
    if (!canRenderPanel(panel)) return
    setError('')
    await renderOne(panel)
  }

  // 全部生成：并发触发所有【已解析且非 done】的格，每格立即显示"生成中"、各自独立完成。
  const renderAll = async () => {
    if (bulkRef.current) return
    bulkRef.current = true
    setBulk(true)
    setError('')
    const renderable = panels.filter((p) => p.status !== 'done' && canRenderPanel(p))
    try {
      await Promise.all(renderable.map((p) => renderOne(p)))
    } finally {
      setBulk(false)
      bulkRef.current = false
    }
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
          还有 {unprocessedCount} 格没解析，点这一格的「🔁 重新解析这格」再出图哦
        </p>
      )}

      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}

      <div className="grid grid-cols-1 items-start gap-5 sm:grid-cols-2">
        {panels.map((panel) => (
          <PanelCard
            key={panel.id}
            panel={panel}
            index={index}
            saving={savingId === panel.id}
            processing={processingId === panel.id}
            canRender={canRenderPanel(panel)}
            onPatch={(patch) => savePanel(panel, patch)}
            onProcess={() => processPanel(panel)}
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
