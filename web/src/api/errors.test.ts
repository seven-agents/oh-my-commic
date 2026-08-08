import { describe, expect, it } from 'vitest'
import { errorMessage } from './errors'
import { ApiError } from './client'

describe('errorMessage', () => {
  it('ApiError 透传其 message', () => {
    expect(errorMessage(new ApiError(404, '找不到这个内容'))).toBe('找不到这个内容')
  })

  it('普通 Error 走友好兜底文案（不泄露原始 message）', () => {
    expect(errorMessage(new Error('TypeError: boom'))).toBe('网络开小差了，待会儿再试试～')
  })

  it('未知值（字符串 / null / undefined）走兜底文案', () => {
    expect(errorMessage('some string')).toBe('网络开小差了，待会儿再试试～')
    expect(errorMessage(null)).toBe('网络开小差了，待会儿再试试～')
    expect(errorMessage(undefined)).toBe('网络开小差了，待会儿再试试～')
  })
})
