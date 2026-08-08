import { Button } from './ui'
import { EditableField } from './EditableField'
import type { Panel } from '../api/types'
import { type AssetIndex, resolveActors, resolveScene } from './chapter/assetLookup'
import { isPanelProcessed } from './chapter/panelStage'

type StoryboardPanelCardProps = {
  panel: Panel
  index: AssetIndex
  saving: boolean
  processing: boolean
  onPatch: (patch: Partial<Panel>) => void
  onProcess: () => void
}

// Stage ①/② 分镜卡：先展示可编辑的基本内容 content；
// 「✨ 解析这格」后展开结构字段（地点/角色/事件/旁白/出图提示词），均可内联编辑。
export function StoryboardPanelCard({
  panel,
  index,
  saving,
  processing,
  onPatch,
  onProcess,
}: StoryboardPanelCardProps) {
  const processed = isPanelProcessed(panel)

  return (
    <div className="flex animate-pop-in flex-col gap-2.5 rounded-4xl bg-white p-4 shadow-soft">
      <div className="flex items-center justify-between">
        <span className="flex h-7 w-7 items-center justify-center rounded-full bg-cream font-display text-sm font-bold text-ink-soft shadow-soft-sm">
          {panel.order + 1}
        </span>
        <span className="font-display text-xs font-semibold text-ink-soft">
          {saving ? '保存中…' : '已保存'}
        </span>
      </div>

      <Section emoji="🎬" label="这一格">
        <EditableField
          value={panel.content}
          placeholder="这一格发生了什么…"
          onCommit={(content) => onPatch({ content })}
        />
      </Section>

      {processed ? (
        <StructuredFields panel={panel} index={index} onPatch={onPatch} />
      ) : (
        <p className="rounded-2xl bg-cream/60 px-3 py-2 text-center text-xs text-ink-soft/70">
          点「✨ 解析这格」，小助手会拆出地点、角色、旁白和出图提示词～
        </p>
      )}

      <Button
        onClick={onProcess}
        loading={processing}
        variant={processed ? 'ghost' : 'primary'}
        className="text-sm"
      >
        {processed ? '🔁 重新解析这格' : '✨ 解析这格'}
      </Button>
      {processed && (
        <p className="text-center text-[11px] text-ink-soft/60">
          重新解析会用当前「这一格」内容覆盖下面的结构字段哦
        </p>
      )}
    </div>
  )
}

type StructuredFieldsProps = {
  panel: Panel
  index: AssetIndex
  onPatch: (patch: Partial<Panel>) => void
}

// 解析后展开的结构字段：地点/出场角色(表情)/场景/事件/旁白/出图提示词，均可内联编辑。
function StructuredFields({ panel, index, onPatch }: StructuredFieldsProps) {
  const actors = resolveActors(panel.characterIds, index)
  const scene = resolveScene(panel.sceneId, index)

  const setExpression = (charId: number, expr: string) =>
    onPatch({ charExpressions: { ...panel.charExpressions, [charId]: expr } })

  const removeActor = (charId: number) => {
    const nextExpr = { ...panel.charExpressions }
    delete nextExpr[charId]
    onPatch({
      characterIds: panel.characterIds.filter((id) => id !== charId),
      charExpressions: nextExpr,
    })
  }

  return (
    <div className="flex flex-col gap-2.5 border-t border-ink/5 pt-2.5">
      <Section emoji="📍" label="地点">
        <EditableField
          value={panel.location}
          placeholder="这一格发生在哪里…"
          onCommit={(location) => onPatch({ location })}
        />
      </Section>

      <Section emoji="🧑" label="出场">
        {actors.length === 0 ? (
          <p className="px-3 py-1.5 text-sm text-ink-soft/60">还没有角色出场～</p>
        ) : (
          <div className="flex flex-col gap-1.5">
            {actors.map((a) => (
              <div key={a.id} className="flex items-center gap-2">
                <span className="inline-flex flex-none items-center gap-1 rounded-full bg-sky/20 px-2.5 py-1 text-xs font-semibold text-sky-deep">
                  {a.emoji} {a.name}
                  <button
                    type="button"
                    onClick={() => removeActor(a.id)}
                    className="ml-0.5 text-sky-deep/60 hover:text-coral"
                    aria-label={`移除 ${a.name}`}
                  >
                    ✕
                  </button>
                </span>
                <EditableField
                  value={panel.charExpressions?.[a.id] ?? ''}
                  placeholder="表情/神态…"
                  onCommit={(expr) => setExpression(a.id, expr)}
                  className="flex-1"
                />
              </div>
            ))}
          </div>
        )}
        {scene && (
          <span className="mt-1 inline-flex w-fit items-center gap-1 rounded-full bg-meadow/25 px-2.5 py-1 text-xs font-semibold text-meadow-deep">
            🏞️ 场景：{scene.name}
          </span>
        )}
      </Section>

      <Section emoji="⚡" label="事件">
        <EditableField
          value={panel.event}
          placeholder="这一格发生了什么…"
          onCommit={(event) => onPatch({ event })}
        />
      </Section>

      <Section emoji="📝" label="旁白">
        <EditableField
          value={panel.caption}
          placeholder="配一句旁白…"
          onCommit={(caption) => onPatch({ caption })}
        />
      </Section>

      <Section emoji="🎨" label="出图提示词">
        <EditableField
          value={panel.imagePrompt}
          placeholder="给画笔的提示词（英文）…"
          onCommit={(imagePrompt) => onPatch({ imagePrompt })}
        />
      </Section>
    </div>
  )
}

function Section({
  emoji,
  label,
  children,
}: {
  emoji: string
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="px-1 font-display text-xs font-bold text-ink-soft">
        {emoji} {label}
      </span>
      {children}
    </div>
  )
}
