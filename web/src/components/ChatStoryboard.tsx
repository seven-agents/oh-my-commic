import { useEffect, useRef, useState } from 'react'
import { Button, Card, LoadingClouds } from './ui'
import { api } from '../api/client'
import type { ChatMessage, Panel } from '../api/types'
import { errorMessage } from '../api/errors'

const INTRO =
  '嗨！我是你的 AI 小助手～给我讲讲你的故事吧，比如：小狐狸和小猫一起去森林找星星～'

const PANEL_OPTIONS = [4, 6, 8] as const
type PanelCount = (typeof PANEL_OPTIONS)[number]

type ChatStoryboardProps = {
  chapterId: string
  onStoryboard: (panels: Panel[]) => void
}

// Stage ① 讲故事：与 AI 对话，攒够故事后一键生成分镜。
export function ChatStoryboard({ chapterId, onStoryboard }: ChatStoryboardProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [panelCount, setPanelCount] = useState<PanelCount>(6)
  const [error, setError] = useState('')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: 'smooth' })
  }, [messages, sending])

  const send = async () => {
    const content = draft.trim()
    if (!content || sending) return
    const next: ChatMessage[] = [...messages, { role: 'user', content }]
    setMessages(next)
    setDraft('')
    setSending(true)
    setError('')
    try {
      const { reply } = await api.post<{ reply: string }>(`/api/chapters/${chapterId}/converse`, {
        messages: next,
      })
      setMessages([...next, { role: 'assistant', content: reply }])
    } catch (err) {
      setError(errorMessage(err))
      setMessages(messages) // 回滚，让用户重发
      setDraft(content)
    } finally {
      setSending(false)
    }
  }

  const generate = async () => {
    if (generating) return
    setGenerating(true)
    setError('')
    try {
      const panels = await api.post<Panel[]>(`/api/chapters/${chapterId}/storyboard`, {
        messages,
        panelCount,
      })
      onStoryboard(panels ?? [])
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setGenerating(false)
    }
  }

  if (generating) {
    return (
      <Card className="flex flex-col items-center">
        <LoadingClouds label="正在把故事排成分镜…可能要等几秒钟哦～" />
      </Card>
    )
  }

  const hasStory = messages.some((m) => m.role === 'user')

  return (
    <Card className="flex flex-col gap-4">
      <div
        ref={listRef}
        className="flex max-h-[420px] min-h-[280px] flex-col gap-3 overflow-y-auto rounded-3xl bg-cream/60 p-4"
      >
        <Bubble role="assistant" content={INTRO} />
        {messages.map((m, i) => (
          <Bubble key={i} role={m.role} content={m.content} />
        ))}
        {sending && (
          <div className="self-start rounded-3xl rounded-bl-lg bg-white px-4 py-3 shadow-soft-sm">
            <span className="inline-flex gap-1" aria-label="正在输入">
              <Dot /> <Dot delay="0.2s" /> <Dot delay="0.4s" />
            </span>
          </div>
        )}
      </div>

      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}

      <div className="flex items-end gap-2">
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              send()
            }
          }}
          rows={2}
          placeholder="讲讲接下来发生了什么…"
          className="field flex-1 resize-none"
        />
        <Button onClick={send} loading={sending} className="flex-none">
          发送
        </Button>
      </div>

      <div className="flex flex-col gap-3 border-t border-ink/5 pt-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold text-ink-soft">分几格？</span>
          {PANEL_OPTIONS.map((n) => (
            <button
              key={n}
              type="button"
              onClick={() => setPanelCount(n)}
              className={[
                'rounded-full px-4 py-1.5 text-sm font-bold transition-all',
                panelCount === n
                  ? 'bg-sky text-white shadow-soft-sm'
                  : 'bg-cream text-ink-soft hover:bg-white',
              ].join(' ')}
            >
              {n} 格
            </button>
          ))}
        </div>
        <Button onClick={generate} disabled={!hasStory} className="text-base">
          ✨ 生成分镜
        </Button>
      </div>
      {!hasStory && (
        <p className="text-center text-xs text-ink-soft/70">先和小助手聊聊你的故事，再生成分镜吧～</p>
      )}
    </Card>
  )
}

function Bubble({ role, content }: { role: ChatMessage['role']; content: string }) {
  const mine = role === 'user'
  return (
    <div className={mine ? 'self-end' : 'self-start'}>
      <div
        className={[
          'max-w-[80%] whitespace-pre-wrap rounded-3xl px-4 py-3 font-body leading-relaxed shadow-soft-sm',
          mine
            ? 'rounded-br-lg bg-gradient-to-b from-peach to-coral text-white'
            : 'rounded-bl-lg bg-white text-ink',
        ].join(' ')}
      >
        {content}
      </div>
    </div>
  )
}

function Dot({ delay = '0s' }: { delay?: string }) {
  return (
    <span
      className="inline-block h-2 w-2 animate-float rounded-full bg-ink-soft/50"
      style={{ animationDelay: delay, animationDuration: '1.2s' }}
    />
  )
}
