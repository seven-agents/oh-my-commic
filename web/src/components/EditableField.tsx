import { useEffect, useRef, useState } from 'react'

type EditableFieldProps = {
  value: string
  placeholder?: string
  onCommit: (next: string) => void
  className?: string
}

// 内联可编辑文本：失焦时若有改动则提交。多行自适应高度。
export function EditableField({ value, placeholder, onCommit, className = '' }: EditableFieldProps) {
  const [draft, setDraft] = useState(value)
  const ref = useRef<HTMLTextAreaElement>(null)

  // 外部值变化（如保存后回填）时同步。
  useEffect(() => {
    setDraft(value)
  }, [value])

  const autoSize = () => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }

  useEffect(autoSize, [draft])

  const commit = () => {
    const trimmed = draft.trim()
    if (trimmed !== value.trim()) onCommit(trimmed)
  }

  return (
    <textarea
      ref={ref}
      rows={1}
      value={draft}
      placeholder={placeholder}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      className={`w-full resize-none rounded-2xl bg-cream/50 px-3 py-1.5 font-body text-sm leading-relaxed text-ink outline-none transition-colors focus:bg-white focus:ring-2 focus:ring-sky/40 ${className}`}
    />
  )
}
