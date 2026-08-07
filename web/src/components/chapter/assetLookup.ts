import type { Character, Scene } from '../../api/types'

// 把角色/场景 id 解析成可展示的名字 + 可爱 emoji。
// 找不到的 id 直接跳过（返回 null）。

export type ResolvedActor = {
  id: number
  name: string
  emoji: string
  imageUrl: string
}

export type ResolvedScene = {
  id: number
  name: string
  imageUrl: string
}

const actorEmoji = (type: Character['type']): string => (type === 'pet' ? '🐾' : '🧒')

export type AssetIndex = {
  characters: Map<number, Character>
  scenes: Map<number, Scene>
}

export function buildAssetIndex(characters: Character[], scenes: Scene[]): AssetIndex {
  return {
    characters: new Map(characters.map((c) => [c.id, c])),
    scenes: new Map(scenes.map((s) => [s.id, s])),
  }
}

export function resolveActors(ids: number[], index: AssetIndex): ResolvedActor[] {
  return ids
    .map((id) => index.characters.get(id))
    .filter((c): c is Character => Boolean(c))
    .map((c) => ({ id: c.id, name: c.name, emoji: actorEmoji(c.type), imageUrl: c.imageUrl }))
}

export function resolveScene(id: number, index: AssetIndex): ResolvedScene | null {
  const s = index.scenes.get(id)
  if (!s) return null
  return { id: s.id, name: s.name, imageUrl: s.imageUrl }
}
