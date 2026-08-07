import type { HTMLAttributes, ReactNode } from 'react'

type CardProps = HTMLAttributes<HTMLDivElement> & {
  children: ReactNode
}

export function Card({ className = '', children, ...rest }: CardProps) {
  return (
    <div className={`card ${className}`} {...rest}>
      {children}
    </div>
  )
}
