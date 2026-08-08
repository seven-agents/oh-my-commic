import { describe, it, expect, beforeEach } from 'vitest'
import { getClientId } from './clientId'

describe('getClientId', () => {
  beforeEach(() => localStorage.clear())

  it('生成并持久化一个稳定 id', () => {
    const a = getClientId()
    expect(a).toBeTruthy()
    const b = getClientId()
    expect(b).toBe(a) // 二次调用返回同一个
    expect(localStorage.getItem('omc.clientId')).toBe(a)
  })
})
