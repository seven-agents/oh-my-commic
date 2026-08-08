import { describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useSubmitOnce } from './useSubmitOnce'

// 手动可控的 Promise，用来把 action 卡在“进行中”状态。
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('useSubmitOnce', () => {
  it('进行中重复调用被同步守卫忽略（action 只跑一次）', async () => {
    const d = deferred<string>()
    const action = vi.fn(() => d.promise)
    const { result } = renderHook(() => useSubmitOnce(action))

    let first!: Promise<string | undefined>
    let second!: Promise<string | undefined>
    act(() => {
      // 同一 tick 内触发两次（模拟双击/回车长按）
      first = result.current.submit()
      second = result.current.submit()
    })

    expect(action).toHaveBeenCalledTimes(1)
    expect(result.current.submitting).toBe(true)

    // 第二次并发调用被忽略，直接 resolve 为 undefined
    await expect(second).resolves.toBeUndefined()

    await act(async () => {
      d.resolve('ok')
      await first
    })

    expect(await first).toBe('ok')
    expect(result.current.submitting).toBe(false)
  })

  it('结束后复位，可再次提交', async () => {
    const action = vi.fn(async () => 'done')
    const { result } = renderHook(() => useSubmitOnce(action))

    await act(async () => {
      await result.current.submit()
    })
    expect(result.current.submitting).toBe(false)

    await act(async () => {
      await result.current.submit()
    })
    expect(action).toHaveBeenCalledTimes(2)
    expect(result.current.submitting).toBe(false)
  })

  it('action 抛错也会复位并把错误抛给调用方', async () => {
    const action = vi.fn(async () => {
      throw new Error('boom')
    })
    const { result } = renderHook(() => useSubmitOnce(action))

    await act(async () => {
      await expect(result.current.submit()).rejects.toThrow('boom')
    })

    // 复位后仍可再次提交
    expect(result.current.submitting).toBe(false)
    await act(async () => {
      await expect(result.current.submit()).rejects.toThrow('boom')
    })
    expect(action).toHaveBeenCalledTimes(2)
  })
})
