const KEY = 'omc.clientId'

// getClientId 返回一个持久化在 localStorage 的匿名访客 id，用于社区浏览量去重。
// 首次调用生成随机 UUID 并落地；后续调用返回同一个。清除缓存会重置（本期可接受）。
export function getClientId(): string {
  try {
    const existing = localStorage.getItem(KEY)
    if (existing) return existing
    const id =
      typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : Math.random().toString(36).slice(2) + Date.now().toString(36)
    localStorage.setItem(KEY, id)
    return id
  } catch {
    // 隐私模式等：退化为进程内随机（不持久，但功能不阻断）。
    return Math.random().toString(36).slice(2)
  }
}
