import type { ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card } from './ui'
import { mediaUrl } from '../api/media'
import type { Character, Scene } from '../api/types'

type AssetPanelProps = {
  bookId: string
  characters: Character[]
  scenes: Scene[]
}

// 演员表：角色 / 宠物 / 场景 三个小分组。
export function AssetPanel({ bookId, characters, scenes }: AssetPanelProps) {
  const navigate = useNavigate()
  const people = characters.filter((c) => c.type === 'character')
  const pets = characters.filter((c) => c.type === 'pet')

  const go = (kind: 'character' | 'pet' | 'scene', assetId?: number) =>
    navigate(`/books/${bookId}/assets/${kind}/${assetId ?? 'new'}`)

  return (
    <Card className="flex flex-col gap-6">
      <h2 className="font-display text-2xl font-extrabold text-ink">演员表 🎭</h2>

      <Section
        title="角色"
        emoji="🧒"
        emptyHint="还没有角色，来添加一个小主角吧～"
        onAdd={() => go('character')}
      >
        {people.map((c) => (
          <AssetTile key={c.id} name={c.name} imageUrl={c.imageUrl} emoji="🧒" onClick={() => go('character', c.id)} />
        ))}
      </Section>

      <Section title="宠物" emoji="🐾" emptyHint="加一只可爱的小宠物吧～" onAdd={() => go('pet')}>
        {pets.map((c) => (
          <AssetTile key={c.id} name={c.name} imageUrl={c.imageUrl} emoji="🐾" onClick={() => go('pet', c.id)} />
        ))}
      </Section>

      <Section title="场景" emoji="🏞️" emptyHint="添加故事发生的地方～" onAdd={() => go('scene')}>
        {scenes.map((s) => (
          <AssetTile key={s.id} name={s.name} imageUrl={s.imageUrl} emoji="🏞️" onClick={() => go('scene', s.id)} />
        ))}
      </Section>
    </Card>
  )
}

function Section({
  title,
  emoji,
  emptyHint,
  onAdd,
  children,
}: {
  title: string
  emoji: string
  emptyHint: string
  onAdd: () => void
  children: ReactNode[]
}) {
  const isEmpty = children.length === 0
  return (
    <section>
      <div className="mb-3 flex items-center justify-between">
        <h3 className="font-display text-lg font-bold text-ink">
          {emoji} {title}
        </h3>
      </div>
      <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
        {children}
        <AddTile onClick={onAdd} />
      </div>
      {isEmpty && <p className="mt-2 text-sm text-ink-soft/70">{emptyHint}</p>}
    </section>
  )
}

function AssetTile({
  name,
  imageUrl,
  emoji,
  onClick,
}: {
  name: string
  imageUrl: string
  emoji: string
  onClick: () => void
}) {
  const src = mediaUrl(imageUrl)
  return (
    <button type="button" onClick={onClick} className="group flex flex-col items-center gap-1.5 text-center">
      <div className="aspect-square w-full overflow-hidden rounded-2xl bg-cream shadow-soft-sm transition-transform group-hover:-translate-y-0.5">
        {src ? (
          <img src={src} alt={name} className="h-full w-full object-cover" loading="lazy" />
        ) : (
          <span className="flex h-full w-full items-center justify-center text-3xl" aria-hidden>
            {emoji}
          </span>
        )}
      </div>
      <span className="w-full truncate text-xs font-semibold text-ink-soft">{name}</span>
    </button>
  )
}

function AddTile({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex aspect-square w-full flex-col items-center justify-center gap-1 rounded-2xl border-2 border-dashed border-ink/15 text-ink-soft transition-all hover:-translate-y-0.5 hover:border-coral/40 hover:text-ink"
    >
      <span className="text-2xl" aria-hidden>
        ＋
      </span>
      <span className="text-xs font-semibold">新建</span>
    </button>
  )
}
