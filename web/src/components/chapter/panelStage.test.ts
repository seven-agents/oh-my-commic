import { describe, expect, it } from 'vitest'
import type { Panel } from '../../api/types'
import { canRenderPanel, isPanelProcessed } from './panelStage'

function panel(overrides: Partial<Panel>): Panel {
  return {
    id: 1,
    chapterId: 1,
    order: 0,
    content: '',
    caption: '',
    characterIds: [],
    sceneId: 0,
    imagePrompt: '',
    imageUrl: '',
    status: 'pending',
    location: '',
    event: '',
    charExpressions: {},
    ...overrides,
  }
}

describe('isPanelProcessed', () => {
  it('结构字段全空时视为未解析', () => {
    expect(isPanelProcessed(panel({}))).toBe(false)
  })

  it('只有空白字符仍视为未解析', () => {
    expect(isPanelProcessed(panel({ caption: '  ', location: '\t', imagePrompt: ' ' }))).toBe(false)
  })

  it('任一结构字段有内容即视为已解析', () => {
    expect(isPanelProcessed(panel({ caption: '旁白' }))).toBe(true)
    expect(isPanelProcessed(panel({ location: '森林' }))).toBe(true)
    expect(isPanelProcessed(panel({ imagePrompt: '一只狐狸' }))).toBe(true)
  })
})

describe('canRenderPanel', () => {
  it('未解析的格不可出图', () => {
    expect(canRenderPanel(panel({}))).toBe(false)
  })

  it('已解析的格可出图', () => {
    expect(canRenderPanel(panel({ imagePrompt: '一只狐狸在森林里' }))).toBe(true)
  })
})
