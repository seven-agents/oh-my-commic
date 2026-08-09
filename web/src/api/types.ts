// API 数据结构，来自 docs/frontend-api.md

export type UserRole = 'admin' | 'user'

export type User = {
  id: number
  username: string
  email: string
  // 权限角色：admin 可管理邀请码，user 为普通用户
  role: UserRole
  // 展示名（问候语等 UI 处使用）
  nickname: string
  age: number
  gender: string
  // 头像图片地址（/media/... 或空串）
  avatarUrl: string
  // 图像积分余额（出图 / 漫画化各扣费，失败退还）
  credits: number
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
  likeCount: number
  viewCount: number
  publishedAt: string
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

// 第1段·讲故事对话的一条消息（持久化在 chapter.conversation）
export type ConversationMsg = {
  role: string
  content: string
}

export type Chapter = {
  id: number
  bookId: number
  order: number
  title: string
  status: ChapterStatus
  summary: string
  isCover: boolean
  createdAt: string
  // 第1段对话历史 + 目标分镜数（重进章节可恢复）
  conversation: ConversationMsg[]
  panelCount: number
}

export type PanelStatus = 'pending' | 'rendering' | 'done' | 'failed'

export type Panel = {
  id: number
  chapterId: number
  order: number
  // 第1段产出的基本分镜内容（“这一格发生了什么”，可直接编辑）
  content: string
  caption: string
  characterIds: number[]
  sceneId: number
  imagePrompt: string
  imageUrl: string
  status: PanelStatus
  location: string
  event: string
  charExpressions: Record<number, string>
}

// AI 对话消息
export type ChatMessage = {
  role: 'user' | 'assistant' | 'system'
  content: string
}

// storyboard-chat 请求体：对话历史 + 可选的目标分镜格数
export type StoryboardChatRequest = {
  messages: ChatMessage[]
  panelCount?: number
}

export interface Author {
  nickname: string
  avatarUrl: string
}

export type CommunitySort = 'new' | 'hot'

export interface CommunityBook {
  id: number
  title: string
  coverUrl: string
  summary: string
  author: Author
  likeCount: number
  viewCount: number
  liked: boolean
  publishedAt: string
}

export interface ReaderChapter {
  title: string
  summary: string
  order: number
  isCover: boolean
  panels: Panel[]
}

export interface CommunityBookDetail {
  id: number
  title: string
  coverUrl: string
  summary: string
  style: string
  author: Author
  likeCount: number
  viewCount: number
  liked: boolean
  chapters: ReaderChapter[]
}

export interface LikeResult {
  likeCount: number
  liked: boolean
}

// 邀请码状态（仅管理员）：当前码 + 已用名额 / 上限（limit 为 0 表示不限制）。
export interface InviteStatus {
  inviteCode: string
  used: number
  limit: number
}
