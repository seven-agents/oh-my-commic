import { useEffect, useRef, useState } from 'react'
import { Button, Card } from './ui'
import { StoryboardPanelList } from './StoryboardPanelList'
import { api } from '../api/client'
import type { ChatMessage, Panel } from '../api/types'
import { errorMessage } from '../api/errors'
import type { AssetIndex } from './chapter/assetLookup'

const INTRO =
  '嗨！我是你的 AI 小助手～给我讲讲你的故事吧，比如：棉花糖在森林里等妈妈回家～'

type StoryboardChatReply = {
  reply: string
  panels: Panel[]
}

type ChatStoryboardProps = {
  chapterId: string
  panels: Panel[]
  index: AssetIndex
  onPanelsChange: (panels: Panel[]) => void
  onConfirm: () => void
}

// Stage ① 讲故事：左侧对话、右侧实时结构化分镜。每轮对话都会返回并持久化全量分镜。
export function ChatStoryboard({
  chapterId,
  panels,
  index,
  onPanelsChange,
  onConfirm,
}: ChatStoryboardProps) {
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
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
      const res = await api.post<StoryboardChatReply>(
        `/api/chapters/${chapterId}/storyboard-chat`,
        { messages: next },
      )
      setMessages([...next, { role: 'assistant', content: res.reply }])
      onPanelsChange(res.panels ?? [])
    } catch (err) {
      setError(errorMessage(err))
      setMessages(messages) // 回滚，让用户重发
      setDraft(content)
    } finally {
      setSending(false)
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <Card className="flex flex-col gap-4">
        <div
          ref={listRef}
          className="flex max-h-[460px] min-h-[300px] flex-col gap-3 overflow-y-auto rounded-3xl bg-cream/60 p-4"
        >
          <Bubble role="assistant" content={INTRO} />
          {messages.map((m, i) => (
            <Bubble key={i} role={m.role} content={m.content} />
          ))}
          {sending && (
            <div className="self-start rounded-3xl rounded-bl-lg bg-white px-4 py-3 shadow-soft-sm">
              <span className="inline-flex gap-1" aria-label="正在思考">
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

        <div className="flex flex-col items-center gap-2 border-t border-ink/5 pt-4">
          <Button onClick={onConfirm} disabled={panels.length === 0} className="text-base">
            ✨ 确认分镜，开始画图 →
          </Button>
          {panels.length === 0 && (
            <p className="text-center text-xs text-ink-soft/70">
              先和小助手聊聊你的故事，分镜会自动出现在右边～
            </p>
          )}
        </div>
      </Card>

      <StoryboardPanelList panels={panels} index={index} onPanelsChange={onPanelsChange} />
    </div>
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
