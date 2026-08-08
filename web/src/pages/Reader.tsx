import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { Button, Card, EmptyState, LoadingClouds } from '../components/ui'
import { ComicPage } from '../components/ComicPage'
import { api } from '../api/client'
import type { Chapter, Panel } from '../api/types'
import { errorMessage } from '../api/errors'

// P6 阅读页：只读展示拼好的漫画书页。
export default function Reader() {
  const { chapterId } = useParams<{ chapterId: string }>()
  const id = chapterId ?? ''

  const [chapter, setChapter] = useState<Chapter | null>(null)
  const [panels, setPanels] = useState<Panel[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    let alive = true
    ;(async () => {
      setLoading(true)
      setError('')
      try {
        const ch = await api.get<Chapter>(`/api/v1/chapters/${id}`)
        const pnls = await api.get<Panel[]>(`/api/v1/chapters/${id}/panels`)
        if (!alive) return
        setChapter(ch)
        setPanels(pnls ?? [])
      } catch (err) {
        if (alive) setError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [id])

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader
        right={
          chapter && (
            <Link to={`/books/${chapter.bookId}`}>
              <Button variant="ghost" className="text-sm">
                ← 返回书房
              </Button>
            </Link>
          )
        }
      />

      <main className="mx-auto max-w-4xl px-6 py-8">
        {loading ? (
          <LoadingClouds label="正在翻开漫画书…" />
        ) : error || !chapter ? (
          <EmptyState
            emoji="🌙"
            title="这本书还没准备好"
            description={error || '找不到这个章节'}
            action={
              <Link to="/my">
                <Button variant="ghost">回到书架</Button>
              </Link>
            }
          />
        ) : (
          <article className="flex flex-col gap-6">
            <header className="text-center">
              <h1 className="font-display text-3xl font-extrabold text-ink">{chapter.title}</h1>
              <p className="mt-1 text-ink-soft">📖 一页一页读你自己的故事～</p>
            </header>
            <Card className="bg-parchment">
              <ComicPage panels={panels} />
            </Card>
            <div className="flex justify-center">
              <Link to={`/books/${chapter.bookId}`}>
                <Button variant="ghost">← 返回书房</Button>
              </Link>
            </div>
          </article>
        )}
      </main>
    </div>
  )
}
