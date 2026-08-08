import { describe, expect, it } from 'vitest'
import { cnNumeral } from './cnNumeral'

describe('cnNumeral', () => {
  it('1–10 映射为「一…十」', () => {
    expect(cnNumeral(1)).toBe('一')
    expect(cnNumeral(2)).toBe('二')
    expect(cnNumeral(5)).toBe('五')
    expect(cnNumeral(9)).toBe('九')
    expect(cnNumeral(10)).toBe('十')
  })

  it('超过 10 用阿拉伯数字兜底', () => {
    expect(cnNumeral(11)).toBe('11')
    expect(cnNumeral(42)).toBe('42')
  })

  it('小于 1 的边界用阿拉伯数字兜底', () => {
    expect(cnNumeral(0)).toBe('0')
    expect(cnNumeral(-1)).toBe('-1')
  })
})
