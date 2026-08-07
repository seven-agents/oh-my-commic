import { useEffect, type ReactNode } from 'react'

type ModalProps = {
  open: boolean
  onClose: () => void
  title?: string
  children: ReactNode
  className?: string
}

export function Modal({ open, onClose, title, children, className = '' }: ModalProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div
        className="absolute inset-0 bg-ink/30 backdrop-blur-sm"
        onClick={onClose}
        aria-hidden
      />
      <div
        className={`relative z-10 w-full max-w-lg animate-pop-in rounded-4xl bg-white p-7 shadow-soft-lg ${className}`}
      >
        {title && <h2 className="mb-4 font-display text-2xl text-ink">{title}</h2>}
        {children}
      </div>
    </div>
  )
}
