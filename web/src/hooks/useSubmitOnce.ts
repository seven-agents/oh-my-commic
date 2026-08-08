import { useCallback, useRef, useState } from 'react'

/**
 * 同步防重入的提交守卫。
 *
 * 背景：用 `setState(submitting)` 做守卫是**异步**的——快速双击或回车长按会在
 * re-render 前重复进入，并发两次 POST，建出重复数据。这里用 `useRef` 立即拦住，
 * `setState` 只负责驱动按钮 loading/disabled 的 UI。
 *
 * 用法：
 *   const { submit, submitting } = useSubmitOnce(async () => { await api.post(...) })
 *   // <Button onClick={submit} loading={submitting} />
 *   // onKeyDown Enter -> submit()
 *
 * 进行中的重复调用会被忽略（返回 undefined）；结束后（无论成功或抛错）复位。
 */
export function useSubmitOnce<T>(
  action: () => Promise<T>,
): { submit: () => Promise<T | undefined>; submitting: boolean } {
  const runningRef = useRef(false)
  const [submitting, setSubmitting] = useState(false)

  const submit = useCallback(async (): Promise<T | undefined> => {
    // 同步守卫：ref 立即置位，抢在 re-render 前拦住并发触发。
    if (runningRef.current) return undefined
    runningRef.current = true
    setSubmitting(true)
    try {
      return await action()
    } finally {
      runningRef.current = false
      setSubmitting(false)
    }
  }, [action])

  return { submit, submitting }
}
