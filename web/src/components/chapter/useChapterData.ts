import { useEffect, useState, type Dispatch, type SetStateAction } from 'react'
import { api } from '../../api/client'
import type { Chapter, Character, Panel, Scene } from '../../api/types'
import { errorMessage } from '../../api/errors'
import { type AssetIndex, buildAssetIndex } from './assetLookup'

export type ChapterData = {
  chapter: Chapter | null
  panels: Panel[]
  index: AssetIndex
  loading: boolean
  error: string
  setPanels: Dispatch<SetStateAction<Panel[]>>
  setChapter: (chapter: Chapter) => void
}

const EMPTY_INDEX: AssetIndex = { characters: new Map(), scenes: new Map() }

// 拉取章节 + 书籍资产 + 分镜，并构建 id→资产 的查找索引。
export function useChapterData(chapterId: string): ChapterData {
  const [chapter, setChapter] = useState<Chapter | null>(null)
  const [panels, setPanels] = useState<Panel[]>([])
  const [index, setIndex] = useState<AssetIndex>(EMPTY_INDEX)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!chapterId) return
    let alive = true
    ;(async () => {
      setLoading(true)
      setError('')
      try {
        const ch = await api.get<Chapter>(`/api/v1/chapters/${chapterId}`)
        const [chars, scenes, pnls] = await Promise.all([
          api.get<Character[]>(`/api/v1/books/${ch.bookId}/characters`),
          api.get<Scene[]>(`/api/v1/books/${ch.bookId}/scenes`),
          api.get<Panel[]>(`/api/v1/chapters/${chapterId}/panels`),
        ])
        if (!alive) return
        setChapter(ch)
        setIndex(buildAssetIndex(chars ?? [], scenes ?? []))
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
  }, [chapterId])

  return { chapter, panels, index, loading, error, setPanels, setChapter }
}
