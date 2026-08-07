import { forwardRef, type TextareaHTMLAttributes } from 'react'

type TextareaProps = TextareaHTMLAttributes<HTMLTextAreaElement> & {
  label?: string
  hint?: string
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { label, hint, id, className = '', ...rest },
  ref,
) {
  return (
    <label className="block" htmlFor={id}>
      {label && <span className="mb-1.5 block px-1 text-sm font-semibold text-ink-soft">{label}</span>}
      <textarea ref={ref} id={id} className={`field resize-none ${className}`} {...rest} />
      {hint && <span className="mt-1.5 block px-1 text-xs text-ink-soft/70">{hint}</span>}
    </label>
  )
})
