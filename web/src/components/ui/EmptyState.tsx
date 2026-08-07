import type { ReactNode } from 'react'

type EmptyStateProps = {
  emoji?: string
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

export function EmptyState({
  emoji = '🌱',
  title,
  description,
  action,
  className = '',
}: EmptyStateProps) {
  return (
    <div className={`flex flex-col items-center gap-3 px-6 py-12 text-center ${className}`}>
      <span className="text-6xl animate-float" aria-hidden>
        {emoji}
      </span>
      <h3 className="font-display text-xl text-ink">{title}</h3>
      {description && <p className="max-w-sm text-ink-soft">{description}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}
