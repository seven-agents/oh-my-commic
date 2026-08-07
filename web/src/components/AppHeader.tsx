import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../auth/useAuth'

type AppHeaderProps = {
  // 面包屑/标题右侧的额外内容（可选）
  right?: ReactNode
}

// 顶部栏：logo + 用户菜单（退出）。所有内页共用。
export function AppHeader({ right }: AppHeaderProps) {
  const { user, logout } = useAuth()
  const [open, setOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setOpen(false)
    }
    window.addEventListener('mousedown', onClick)
    return () => window.removeEventListener('mousedown', onClick)
  }, [open])

  const initial = user?.nickname?.trim()?.[0] ?? '🙂'

  return (
    <header className="sticky top-0 z-30 flex items-center justify-between gap-3 border-b border-ink/5 bg-cream/80 px-6 py-4 backdrop-blur">
      <Link to="/" className="font-display text-2xl font-extrabold text-ink">
        🎨 oh-my-commic
      </Link>

      <div className="flex items-center gap-3">
        {right}
        <div className="relative" ref={menuRef}>
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            className="flex h-11 w-11 items-center justify-center rounded-full bg-gradient-to-b from-sun to-peach font-display text-lg font-bold text-white shadow-soft-sm transition-transform hover:-translate-y-0.5"
            aria-label="用户菜单"
          >
            {initial}
          </button>
          {open && (
            <div className="absolute right-0 mt-2 w-44 animate-pop-in rounded-3xl bg-white p-2 shadow-soft-lg">
              {user && (
                <p className="truncate px-3 py-2 text-sm font-semibold text-ink-soft">
                  你好，{user.nickname} 👋
                </p>
              )}
              <button
                type="button"
                onClick={() => logout()}
                className="w-full rounded-2xl px-3 py-2.5 text-left font-display font-semibold text-coral hover:bg-coral/10"
              >
                退出
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}
