// 可爱的加载动画：漂浮的云朵与眨眼的小星星。
type LoadingCloudsProps = {
  label?: string
  className?: string
}

export function LoadingClouds({ label = '正在画画…', className = '' }: LoadingCloudsProps) {
  return (
    <div className={`flex flex-col items-center gap-4 py-8 ${className}`}>
      <div className="relative h-16 w-40" aria-hidden>
        <span className="absolute left-2 top-4 text-4xl animate-float-slow">☁️</span>
        <span className="absolute left-14 top-0 text-5xl animate-float">☁️</span>
        <span className="absolute right-2 top-6 text-3xl animate-float-slow">☁️</span>
        <span className="absolute left-9 top-2 text-lg animate-twinkle">✨</span>
        <span
          className="absolute right-10 top-0 text-base animate-twinkle"
          style={{ animationDelay: '0.6s' }}
        >
          ⭐️
        </span>
      </div>
      {label && <p className="font-display text-ink-soft">{label}</p>}
    </div>
  )
}
