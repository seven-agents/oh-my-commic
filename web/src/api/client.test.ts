// 轻测：验证用户管理相关的 client 方法打到正确的 /api/v1 路径、携带 cookie 与正确请求体。
// 通过 mock 全局 fetch 断言调用参数，不发真实网络请求。
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './client'

// 返回一个成功的 JSON 响应（200 + 指定 body）。
function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mockFetch(body: unknown) {
  // 显式声明 fetch 签名，使 mock.calls[i] 被推断为 [string, RequestInit]。
  const spy = vi.fn(async (_path: string, _init: RequestInit): Promise<Response> =>
    jsonResponse(body),
  )
  vi.stubGlobal('fetch', spy)
  return spy
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('api 用户管理方法', () => {
  it('register 打到 /api/v1/register 且带完整 body（含 credentials）', async () => {
    const spy = mockFetch({ id: 1, username: 'kid' })
    await api.register({
      username: 'kid',
      password: 'secret6',
      email: 'kid@example.com',
      inviteCode: 'CODE',
      nickname: '小明',
    })

    expect(spy).toHaveBeenCalledTimes(1)
    const [path, init] = spy.mock.calls[0]
    expect(path).toBe('/api/v1/register')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(JSON.parse(init.body as string)).toEqual({
      username: 'kid',
      password: 'secret6',
      email: 'kid@example.com',
      inviteCode: 'CODE',
      nickname: '小明',
    })
  })

  it('login 打到 /api/v1/login 且带 username/password', async () => {
    const spy = mockFetch({ id: 1, username: 'kid' })
    await api.login({ username: 'kid', password: 'secret6' })

    const [path, init] = spy.mock.calls[0]
    expect(path).toBe('/api/v1/login')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({
      username: 'kid',
      password: 'secret6',
    })
  })

  it('updateProfile 用 PUT 打到 /api/v1/me/profile', async () => {
    const spy = mockFetch({ id: 1 })
    await api.updateProfile({ nickname: '小明', age: 7, gender: 'boy' })

    const [path, init] = spy.mock.calls[0]
    expect(path).toBe('/api/v1/me/profile')
    expect(init.method).toBe('PUT')
    expect(JSON.parse(init.body as string)).toEqual({
      nickname: '小明',
      age: 7,
      gender: 'boy',
    })
  })

  it('uploadAvatar 用 multipart（FormData，字段名 file）POST 到 /api/v1/me/avatar', async () => {
    const spy = mockFetch({ id: 1, avatarUrl: '/media/users/1/a.png' })
    const file = new File(['x'], 'a.png', { type: 'image/png' })
    await api.uploadAvatar(file)

    const [path, init] = spy.mock.calls[0]
    expect(path).toBe('/api/v1/me/avatar')
    expect(init.method).toBe('POST')
    expect(init.credentials).toBe('include')
    expect(init.body).toBeInstanceOf(FormData)
    expect((init.body as FormData).get('file')).toBeInstanceOf(File)
  })

  it('getInviteCode / rotateInviteCode 打到 admin 邀请码端点', async () => {
    const getSpy = mockFetch({ inviteCode: 'ABC' })
    await api.getInviteCode()
    let [path, init] = getSpy.mock.calls[0]
    expect(path).toBe('/api/v1/admin/invite-code')
    expect(init.method).toBe('GET')

    const postSpy = mockFetch({ inviteCode: 'XYZ' })
    await api.rotateInviteCode()
    ;[path, init] = postSpy.mock.calls[0]
    expect(path).toBe('/api/v1/admin/invite-code/rotate')
    expect(init.method).toBe('POST')
  })
})
