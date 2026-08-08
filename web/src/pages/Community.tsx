import { useEffect, useRef, useState } from 'react'
import { AppHeader } from '../components/AppHeader'
import { CommunityCard } from '../components/CommunityCard'
import { Button, EmptyState, LoadingClouds } from '../components/ui'
import { api } from '../api/client'
import type { CommunityBook } from '../api/types'
import { errorMessage } from '../api/errors'

const PAGE_SIZE = 20

// 社区 feed：公开漫画网格 + 「加载更多」分页。任何人（含未登录）可访问。
export default function Community() {
  const [items, setItems] = useState<CommunityBook[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [offset, setOffset] = useState(0)
  const [done, setDone] = useState(false)
  // 防止首屏加载与「加载更多」重复触发请求
  const busyRef = useRef(false)

  useEffect(() => {
    let alive = true
    busyRef.current = true
    ;(async () => {
      try {
        const data = await api.listCommunity({ limit: PAGE_SIZE, offset: 0 })
        if (!alive) return
        const list = data ?? []
        setItems(list)
        setOffset(list.length)
        setDone(list.length < PAGE_SIZE)
      } catch (err) {
        if (alive) setError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
        busyRef.current = false
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  const [loadingMore, setLoadingMore] = useState(false)

  const loadMore = async () => {
    if (busyRef.current || done) return
    busyRef.current = true
    setLoadingMore(true)
    setError('')
    try {
      const data = await api.listCommunity({ limit: PAGE_SIZE, offset })
      const list = data ?? []
      setItems((prev) => [...prev, ...list])
      setOffset((prev) => prev + list.length)
      if (list.length < PAGE_SIZE) setDone(true)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setLoadingMore(false)
      busyRef.current = false
    }
  }

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader />

      <main className="mx-auto max-w-6xl px-6 py-8">
        <div className="mb-6">
          <h1 className="font-display text-3xl font-extrabold text-ink">社区 🌈</h1>
          <p className="mt-1 text-ink-soft">看看大家画的漫画书吧～</p>
        </div>

        {loading ? (
          <LoadingClouds label="正在打开社区…" />
        ) : error && items.length === 0 ? (
          <EmptyState emoji="🌧️" title="社区没打开" description={error} />
        ) : items.length === 0 ? (
          <EmptyState
            emoji="🎨"
            title="还没有公开的漫画"
            description="还没有公开的漫画，快去发布第一本吧～"
          />
        ) : (
          <>
            <div className="grid grid-cols-2 gap-5 sm:grid-cols-3 lg:grid-cols-4">
              {items.map((book) => (
                <CommunityCard key={book.id} book={book} />
              ))}
            </div>

            {error && (
              <p className="mt-6 text-center text-sm font-semibold text-coral">{error}</p>
            )}

            {!done && (
              <div className="mt-8 flex justify-center">
                <Button variant="ghost" onClick={loadMore} loading={loadingMore}>
                  加载更多
                </Button>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  )
}
