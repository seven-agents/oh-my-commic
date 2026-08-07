import { ApiError } from './client'

// 统一把未知错误转成给小朋友看的友好文案。
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  return '网络开小差了，待会儿再试试～'
}
