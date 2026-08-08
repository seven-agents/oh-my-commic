import { useEffect, useRef, useState } from 'react'
import { CommunityCard } from '../components/CommunityCard'
import { HeroBanner } from '../components/community/HeroBanner'
import { SortToggle } from '../components/community/SortToggle'
import { FeaturedRow } from '../components/community/FeaturedRow'
import { Button, EmptyState, LoadingClouds } from '../components/ui'
import { api } from '../api/client'
import type { CommunityBook, CommunitySort } from '../api/types'
import { errorMessage } from '../api/errors'

const PAGE_SIZE = 20
const FEATURED_N = 3

// 社区内容区：欢迎横幅 + 精选(最热3) + 排序切换 + 卡片网格 + 空态。
export default function CommunityView() {
  const [featured, setFeatured] = useState<CommunityBook[]>([])
  const [showFeatured, setShowFeatured] = useState(false)
  const [items, setItems] = useState<CommunityBook[]>([])
  const [sort, setSort] = useState<CommunitySort>('new')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [offset, setOffset] = useState(0)
  const [done, setDone] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const busyRef = useRef(false)

  // 首屏：并行拉精选(最热3)与网格(当前 sort)。
  useEffect(() => {
    let alive = true
    ;(async () => {
      setLoading(true)
      const featuredP = api
        .listCommunity({ sort: 'hot', limit: FEATURED_N })
        .catch(() => [] as CommunityBook[]) // 精选失败静默隐藏，不阻断网格
      const gridP = api.listCommunity({ sort, limit: PAGE_SIZE, offset: 0 })
      try {
        const [feat, grid] = await Promise.all([featuredP, gridP])
        if (!alive) return
        const gridList = grid ?? []
        const show = (feat ?? []).length === FEATURED_N && gridList.length > FEATURED_N
        setFeatured(feat ?? [])
        setShowFeatured(show)
        setItems(gridList)
        setOffset(gridList.length)
        setDone(gridList.length < PAGE_SIZE)
      } catch (err) {
        if (alive) setError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
    // 仅首屏拉一次；排序切换走 changeSort 单独重拉网格。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const changeSort = async (next: CommunitySort) => {
    if (next === sort) return
    setSort(next)
    setError('')
    busyRef.current = true
    try {
      const grid = await api.listCommunity({ sort: next, limit: PAGE_SIZE, offset: 0 })
      const list = grid ?? []
      setItems(list)
      setOffset(list.length)
      setDone(list.length < PAGE_SIZE)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      busyRef.current = false
    }
  }

  const loadMore = async () => {
    if (busyRef.current || done) return
    busyRef.current = true
    setLoadingMore(true)
    setError('')
    try {
      const grid = await api.listCommunity({ sort, limit: PAGE_SIZE, offset })
      const list = grid ?? []
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

  const featuredIds = new Set(featured.map((b) => b.id))
  const gridItems = showFeatured ? items.filter((b) => !featuredIds.has(b.id)) : items

  return (
    <div className="mx-auto max-w-6xl px-6 py-8">
      <HeroBanner />

      {loading ? (
        <div className="mt-8">
          <LoadingClouds label="正在打开社区…" />
        </div>
      ) : error && items.length === 0 ? (
        <div className="mt-8">
          <EmptyState emoji="🌧️" title="社区没打开" description={error} />
        </div>
      ) : items.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            emoji="🎨"
            title="还没有公开的漫画"
            description="还没有公开的漫画，去创作并发布第一本吧～"
          />
        </div>
      ) : (
        <>
          {showFeatured && (
            <section className="mt-8">
              <h2 className="mb-3 font-display text-xl font-extrabold text-ink">✨ 精选</h2>
              <FeaturedRow books={featured} />
            </section>
          )}

          <div className="mt-8 flex items-center justify-between">
            <h2 className="font-display text-xl font-extrabold text-ink">全部作品</h2>
            <SortToggle value={sort} onChange={changeSort} />
          </div>

          <div className="mt-4 grid grid-cols-2 gap-5 sm:grid-cols-3 lg:grid-cols-4">
            {gridItems.map((book) => (
              <CommunityCard key={book.id} book={book} />
            ))}
          </div>

          {error && <p className="mt-6 text-center text-sm font-semibold text-coral">{error}</p>}

          {!done && (
            <div className="mt-8 flex justify-center">
              <Button variant="ghost" onClick={loadMore} loading={loadingMore}>
                加载更多
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
