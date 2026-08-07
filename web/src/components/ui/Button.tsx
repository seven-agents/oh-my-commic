import type { ButtonHTMLAttributes, ReactNode } from 'react'

type Variant = 'primary' | 'ghost'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant
  loading?: boolean
  children: ReactNode
}

const variantClass: Record<Variant, string> = {
  primary: 'btn-primary',
  ghost: 'btn-ghost',
}

export function Button({
  variant = 'primary',
  loading = false,
  disabled,
  className = '',
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      className={`${variantClass[variant]} ${className}`}
      disabled={disabled || loading}
      {...rest}
    >
      {loading && (
        <span
          className="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white/50 border-t-white"
          aria-hidden
        />
      )}
      {children}
    </button>
  )
}
