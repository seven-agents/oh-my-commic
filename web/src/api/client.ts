// 轻量 fetch 封装。所有路径已包含 /api 前缀，base 为空字符串。
// 始终携带 cookie（credentials: 'include'）。非 2xx 抛出带类型的 ApiError。

import type { User } from './types'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// 全局 401 处理：清除登录态并跳转到 /login。
// 由 auth 层通过 setUnauthorizedHandler 注册，避免 client 直接依赖 React/router。
type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler | null = null

export function setUnauthorizedHandler(handler: UnauthorizedHandler | null) {
  onUnauthorized = handler
}

async function parseError(res: Response): Promise<string> {
  try {
    const data = await res.json()
    if (data && typeof data.error === 'string') return data.error
  } catch {
    // body 不是 JSON，忽略
  }
  return defaultMessage(res.status)
}

function defaultMessage(status: number): string {
  switch (status) {
    case 400:
      return '请求有点小问题，检查一下再试试～'
    case 401:
      return '需要先登录哦'
    case 402:
      return '积分不足啦，画不了新图咯～'
    case 404:
      return '找不到这个内容'
    case 502:
      return '画笔累坏啦，稍后再试试～'
    default:
      return '出了点小状况，待会儿再来～'
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
  }
  if (body !== undefined) init.body = JSON.stringify(body)

  const res = await fetch(path, init)
  return handleResponse<T>(res)
}

async function handleResponse<T>(res: Response): Promise<T> {
  if (res.status === 401 && onUnauthorized) onUnauthorized()

  if (!res.ok) {
    throw new ApiError(res.status, await parseError(res))
  }

  // 204 或空响应
  if (res.status === 204) return undefined as T
  const text = await res.text()
  if (!text) return undefined as T
  return JSON.parse(text) as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),

  // 拉取当前登录用户（含最新积分余额），用于登录后 / 出图 / 漫画化后刷新 header。
  getMe: () => request<User>('GET', '/api/me'),

  // multipart 上传：不要手动设置 Content-Type，交给浏览器带 boundary。
  upload: async <T>(path: string, file: File): Promise<T> => {
    const form = new FormData()
    form.append('file', file)
    const res = await fetch(path, {
      method: 'POST',
      credentials: 'include',
      body: form,
    })
    return handleResponse<T>(res)
  },
}
