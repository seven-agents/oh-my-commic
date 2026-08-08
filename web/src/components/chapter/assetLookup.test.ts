import { describe, expect, it } from 'vitest'
import type { Character, Scene } from '../../api/types'
import { buildAssetIndex, resolveActors, resolveScene } from './assetLookup'

function char(overrides: Partial<Character> & { id: number }): Character {
  return {
    bookId: 1,
    type: 'character',
    name: '小明',
    gender: '',
    age: '',
    personality: '',
    description: '',
    imageUrl: '/media/c.png',
    ...overrides,
  }
}

function scene(overrides: Partial<Scene> & { id: number }): Scene {
  return {
    bookId: 1,
    name: '森林',
    description: '',
    imageUrl: '/media/s.png',
    ...overrides,
  }
}

describe('resolveActors', () => {
  const index = buildAssetIndex(
    [
      char({ id: 1, name: '小明', type: 'character' }),
      char({ id: 2, name: '旺财', type: 'pet' }),
    ],
    [],
  )

  it('按 id 解析出名字与 emoji（人 🧒 / 宠物 🐾）', () => {
    const actors = resolveActors([1, 2], index)
    expect(actors).toEqual([
      { id: 1, name: '小明', emoji: '🧒', imageUrl: '/media/c.png' },
      { id: 2, name: '旺财', emoji: '🐾', imageUrl: '/media/c.png' },
    ])
  })

  it('跳过找不到的 id', () => {
    const actors = resolveActors([1, 999], index)
    expect(actors.map((a) => a.id)).toEqual([1])
  })

  it('空 id 列表返回空数组', () => {
    expect(resolveActors([], index)).toEqual([])
  })
})

describe('resolveScene', () => {
  const index = buildAssetIndex([], [scene({ id: 10, name: '森林' })])

  it('命中的场景返回 {id,name,imageUrl}', () => {
    expect(resolveScene(10, index)).toEqual({ id: 10, name: '森林', imageUrl: '/media/s.png' })
  })

  it('未命中的场景返回 null', () => {
    expect(resolveScene(404, index)).toBeNull()
  })
})
