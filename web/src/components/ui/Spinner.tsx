type SpinnerProps = {
  size?: number
  className?: string
}

export function Spinner({ size = 24, className = '' }: SpinnerProps) {
  return (
    <span
      role="status"
      aria-label="加载中"
      className={`inline-block animate-spin rounded-full border-[3px] border-sun/30 border-t-coral ${className}`}
      style={{ width: size, height: size }}
    />
  )
}
