import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Button, Card } from './ui'
import { ComicPage } from './ComicPage'
import { api } from '../api/client'
import type { Chapter, Panel } from '../api/types'
import { errorMessage } from '../api/errors'

type ComicComposeProps = {
  chapterId: string
  title: string
  panels: Panel[]
  onSaved: (chapter: Chapter) => void
  // 封面模式：文案改为「保存封面」（cover_url 已在渲染时同步）。
  coverMode?: boolean
}

// Stage ③ 拼成书：预览整页漫画 + 保存成书。
export function ComicCompose({
  chapterId,
  title,
  panels,
  onSaved,
  coverMode = false,
}: ComicComposeProps) {
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const save = async () => {
    setSaving(true)
    setError('')
    try {
      const ch = await api.put<Chapter>(`/api/chapters/${chapterId}/status`, { status: 'done' })
      onSaved(ch)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <Card className="flex flex-col gap-6 bg-parchment">
        <h2 className="text-center font-display text-2xl font-extrabold text-ink">{title}</h2>
        <ComicPage panels={panels} />
      </Card>

      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}

      <div className="flex flex-wrap items-center justify-center gap-3">
        <Button onClick={save} loading={saving} className="text-base">
          {coverMode ? '🎨 保存封面' : '📖 保存成书'}
        </Button>
        {!coverMode && (
          <Link to={`/read/${chapterId}`}>
            <Button variant="ghost" className="text-base">
              去阅读
            </Button>
          </Link>
        )}
      </div>
    </div>
  )
}
