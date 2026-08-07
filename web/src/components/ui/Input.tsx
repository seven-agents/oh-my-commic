import { forwardRef, type InputHTMLAttributes } from 'react'

type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  label?: string
  hint?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { label, hint, id, className = '', ...rest },
  ref,
) {
  return (
    <label className="block" htmlFor={id}>
      {label && <span className="mb-1.5 block px-1 text-sm font-semibold text-ink-soft">{label}</span>}
      <input ref={ref} id={id} className={`field ${className}`} {...rest} />
      {hint && <span className="mt-1.5 block px-1 text-xs text-ink-soft/70">{hint}</span>}
    </label>
  )
})
