import { describe, expect, it } from 'vitest'
import { mediaUrl } from './media'

describe('mediaUrl', () => {
  it('原样返回相对 /media 路径', () => {
    expect(mediaUrl('/media/books/1/cover.png')).toBe('/media/books/1/cover.png')
  })

  it('原样返回绝对地址', () => {
    expect(mediaUrl('https://cdn.example.com/a.png')).toBe('https://cdn.example.com/a.png')
  })

  it('空字符串返回空串', () => {
    expect(mediaUrl('')).toBe('')
  })

  it('undefined / null 返回空串', () => {
    expect(mediaUrl(undefined)).toBe('')
    expect(mediaUrl(null)).toBe('')
  })
})
