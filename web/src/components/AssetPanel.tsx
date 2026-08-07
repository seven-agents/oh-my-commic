import { useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Modal } from './ui'
import { mediaUrl } from '../api/media'
import { api } from '../api/client'
import { errorMessage } from '../api/errors'
import type { Character, Scene } from '../api/types'

type AssetKind = 'character' | 'pet' | 'scene'

// 待删除资产：区分角色/宠物走 characters 端点、场景走 scenes 端点。
type PendingDelete = {
  kind: AssetKind
  id: number
  name: string
}

type AssetPanelProps = {
  bookId: string
  characters: Character[]
  scenes: Scene[]
  onAssetsChanged: () => void | Promise<void>
}

// 演员表：角色 / 宠物 / 场景 三个小分组。
export function AssetPanel({ bookId, characters, scenes, onAssetsChanged }: AssetPanelProps) {
  const navigate = useNavigate()
  const people = characters.filter((c) => c.type === 'character')
  const pets = characters.filter((c) => c.type === 'pet')

  const [pending, setPending] = useState<PendingDelete | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState('')

  const go = (kind: AssetKind, assetId?: number) =>
    navigate(`/books/${bookId}/assets/${kind}/${assetId ?? 'new'}`)

  const askDelete = (kind: AssetKind, id: number, name: string) => {
    setDeleteError('')
    setPending({ kind, id, name })
  }

  const closeConfirm = () => {
    if (deleting) return
    setPending(null)
    setDeleteError('')
  }

  const confirmDelete = async () => {
    if (!pending) return
    setDeleting(true)
    setDeleteError('')
    try {
      const path =
        pending.kind === 'scene'
          ? `/api/books/${bookId}/scenes/${pending.id}`
          : `/api/books/${bookId}/characters/${pending.id}`
      await api.del(path)
      await onAssetsChanged()
      setPending(null)
    } catch (err) {
      setDeleteError(errorMessage(err))
    } finally {
      setDeleting(false)
    }
  }

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
          <AssetTile
            key={c.id}
            name={c.name}
            imageUrl={c.imageUrl}
            emoji="🧒"
            onClick={() => go('character', c.id)}
            onDelete={() => askDelete('character', c.id, c.name)}
          />
        ))}
      </Section>

      <Section title="宠物" emoji="🐾" emptyHint="加一只可爱的小宠物吧～" onAdd={() => go('pet')}>
        {pets.map((c) => (
          <AssetTile
            key={c.id}
            name={c.name}
            imageUrl={c.imageUrl}
            emoji="🐾"
            onClick={() => go('pet', c.id)}
            onDelete={() => askDelete('pet', c.id, c.name)}
          />
        ))}
      </Section>

      <Section title="场景" emoji="🏞️" emptyHint="添加故事发生的地方～" onAdd={() => go('scene')}>
        {scenes.map((s) => (
          <AssetTile
            key={s.id}
            name={s.name}
            imageUrl={s.imageUrl}
            emoji="🏞️"
            onClick={() => go('scene', s.id)}
            onDelete={() => askDelete('scene', s.id, s.name)}
          />
        ))}
      </Section>

      <Modal open={pending !== null} onClose={closeConfirm} title="删掉这个吗？">
        {pending && (
          <div className="flex flex-col gap-5">
            <p className="text-ink-soft">
              确定删掉「<span className="font-bold text-ink">{pending.name}</span>」吗？删掉就找不回来啦～
            </p>
            {deleteError && <p className="text-sm text-coral">{deleteError}</p>}
            <div className="flex justify-end gap-3">
              <Button variant="ghost" onClick={closeConfirm} disabled={deleting}>
                取消
              </Button>
              <Button
                onClick={confirmDelete}
                loading={deleting}
                className="bg-coral text-white hover:bg-coral"
              >
                删除
              </Button>
            </div>
          </div>
        )}
      </Modal>
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
  onDelete,
}: {
  name: string
  imageUrl: string
  emoji: string
  onClick: () => void
  onDelete: () => void
}) {
  const src = mediaUrl(imageUrl)
  return (
    <div className="group relative">
      <button type="button" onClick={onClick} className="flex w-full flex-col items-center gap-1.5 text-center">
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
      <button
        type="button"
        aria-label={`删除${name}`}
        onClick={(e) => {
          e.stopPropagation()
          onDelete()
        }}
        className="absolute right-1 top-1 flex h-6 w-6 items-center justify-center rounded-full bg-white/90 text-sm text-coral shadow-soft-sm transition-all hover:bg-coral hover:text-white focus:opacity-100 group-hover:opacity-100 sm:opacity-0"
      >
        <span aria-hidden>✕</span>
      </button>
    </div>
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
