import { EditableField } from './EditableField'
import type { Panel } from '../api/types'
import { type AssetIndex, resolveActors, resolveScene } from './chapter/assetLookup'

type StoryboardPanelCardProps = {
  panel: Panel
  index: AssetIndex
  saving: boolean
  onPatch: (patch: Partial<Panel>) => void
}

// Stage ① 结构化分镜卡片：地点/出场角色(表情)/事件/旁白，均可内联编辑。
export function StoryboardPanelCard({ panel, index, saving, onPatch }: StoryboardPanelCardProps) {
  const actors = resolveActors(panel.characterIds, index)
  const scene = resolveScene(panel.sceneId, index)

  const setExpression = (charId: number, expr: string) =>
    onPatch({ charExpressions: { ...panel.charExpressions, [charId]: expr } })

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
