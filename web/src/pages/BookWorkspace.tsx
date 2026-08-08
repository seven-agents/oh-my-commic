import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { AssetPanel } from '../components/AssetPanel'
import { ChapterList } from '../components/ChapterList'
import { Button, EmptyState, LoadingClouds } from '../components/ui'
import { api } from '../api/client'
import type { Book, Chapter, Character, Scene } from '../api/types'
import { errorMessage } from '../api/errors'

export default function BookWorkspace() {
  const { id } = useParams<{ id: string }>()
  const bookId = id ?? ''

  const [book, setBook] = useState<Book | null>(null)
  const [characters, setCharacters] = useState<Character[]>([])
  const [scenes, setScenes] = useState<Scene[]>([])
  const [chapters, setChapters] = useState<Chapter[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    if (!bookId) return
    let alive = true
    ;(async () => {
      setLoading(true)
      setLoadError('')
      try {
        const [b, chars, scs, chaps] = await Promise.all([
          api.get<Book>(`/api/books/${bookId}`),
          api.get<Character[]>(`/api/books/${bookId}/characters`),
          api.get<Scene[]>(`/api/books/${bookId}/scenes`),
          api.get<Chapter[]>(`/api/books/${bookId}/chapters`),
        ])
        if (!alive) return
        setBook(b)
        setCharacters(chars ?? [])
        setScenes(scs ?? [])
        setChapters(chaps ?? [])
      } catch (err) {
        if (alive) setLoadError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [bookId])

  // 删除资产后重新拉取角色 + 场景，让卡片消失。
  const refetchAssets = useCallback(async () => {
    if (!bookId) return
    const [chars, scs] = await Promise.all([
      api.get<Character[]>(`/api/books/${bookId}/characters`),
      api.get<Scene[]>(`/api/books/${bookId}/scenes`),
    ])
    setCharacters(chars ?? [])
    setScenes(scs ?? [])
  }, [bookId])

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader
        right={
          <Link to="/">
            <Button variant="ghost" className="text-sm">
              ← 书架
            </Button>
          </Link>
        }
      />

      <main className="mx-auto max-w-6xl px-6 py-8">
        {loading ? (
          <LoadingClouds label="正在翻开这本书…" />
        ) : loadError || !book ? (
          <EmptyState
            emoji="🌧️"
            title="这本书没打开"
            description={loadError || '找不到这本书'}
            action={
              <Link to="/">
                <Button variant="ghost">回到书架</Button>
              </Link>
            }
          />
        ) : (
          <>
            <header className="mb-6">
              <h1 className="font-display text-3xl font-extrabold text-ink">{book.title}</h1>
              {book.summary && <p className="mt-1 text-ink-soft">{book.summary}</p>}
            </header>

            <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
              <AssetPanel
                bookId={bookId}
                characters={characters}
                scenes={scenes}
                onAssetsChanged={refetchAssets}
              />
              <ChapterList
                book={book}
                chapters={chapters}
                onCreated={(ch) =>
                  setChapters((prev) =>
                    prev.some((c) => c.id === ch.id) ? prev : [...prev, ch],
                  )
                }
              />
            </div>
          </>
        )}
      </main>
    </div>
  )
}
