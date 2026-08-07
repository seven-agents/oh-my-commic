// API 数据结构，来自 docs/frontend-api.md

export type User = {
  id: number
  nickname: string
  createdAt: string
}

export type Book = {
  id: number
  userId: number
  title: string
  coverUrl: string
  style: string
  summary: string
  isPublic: boolean
  createdAt: string
  updatedAt: string
}

export type CharacterType = 'character' | 'pet'

export type Character = {
  id: number
  bookId: number
  type: CharacterType
  name: string
  gender: string
  age: string
  personality: string
  description: string
  imageUrl: string
}

export type Scene = {
  id: number
  bookId: number
  name: string
  description: string
  imageUrl: string
}

export type ChapterStatus = 'draft' | 'storyboarding' | 'rendering' | 'done'

export type Chapter = {
  id: number
  bookId: number
  order: number
  title: string
  status: ChapterStatus
  createdAt: string
}

export type PanelStatus = 'pending' | 'rendering' | 'done' | 'failed'

export type Panel = {
  id: number
  chapterId: number
  order: number
  caption: string
  characterIds: number[]
  sceneId: number
  imagePrompt: string
  imageUrl: string
  status: PanelStatus
}

// AI 对话消息
export type ChatMessage = {
  role: 'user' | 'assistant' | 'system'
  content: string
}
