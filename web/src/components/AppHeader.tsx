import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../auth/useAuth'
import { mediaUrl } from '../api/media'

type AppHeaderProps = {
  // 面包屑/标题右侧的额外内容（可选）
  right?: ReactNode
}

// 顶部栏：logo + 用户菜单（退出）。所有内页共用。
export function AppHeader({ right }: AppHeaderProps) {
  const { user, isAuthed, logout } = useAuth()
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
  const avatarUrl = mediaUrl(user?.avatarUrl)

  return (
    <header className="sticky top-0 z-30 flex items-center justify-between gap-3 border-b border-ink/5 bg-cream/80 px-6 py-4 backdrop-blur">
      <Link to="/" className="font-display text-2xl font-extrabold text-ink">
        🎨 oh-my-commic
      </Link>

      <div className="flex items-center gap-3">
        {right}
        {isAuthed && user ? (
          <>
            <span
              className="flex items-center gap-1 rounded-full bg-white/70 px-3 py-1.5 font-display text-sm font-bold text-ink-soft shadow-soft-sm"
              title="图像积分：出图 / 漫画化各扣 1 分，失败退还"
            >
              ⭐ 积分 {user.credits}
            </span>
            <div className="relative" ref={menuRef}>
              <button
                type="button"
                onClick={() => setOpen((v) => !v)}
                className="flex h-11 w-11 items-center justify-center overflow-hidden rounded-full bg-gradient-to-b from-sun to-peach font-display text-lg font-bold text-white shadow-soft-sm transition-transform hover:-translate-y-0.5"
                aria-label="用户菜单"
              >
                {avatarUrl ? (
                  <img src={avatarUrl} alt="头像" className="h-full w-full object-cover" />
                ) : (
                  initial
                )}
              </button>
              {open && (
                <div className="absolute right-0 mt-2 w-44 animate-pop-in rounded-3xl bg-white p-2 shadow-soft-lg">
                  <p className="truncate px-3 py-2 text-sm font-semibold text-ink-soft">
                    你好，{user.nickname} 👋
                  </p>
                  <Link
                    to="/profile"
                    onClick={() => setOpen(false)}
                    className="block w-full rounded-2xl px-3 py-2.5 text-left font-display font-semibold text-ink-soft hover:bg-sky/10"
                  >
                    个人资料
                  </Link>
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
          </>
        ) : (
          <Link
            to="/login"
            className="rounded-full bg-gradient-to-b from-sun to-peach px-5 py-2.5 font-display text-sm font-bold text-white shadow-soft-sm transition-transform hover:-translate-y-0.5"
          >
            登录 / 注册
          </Link>
        )}
      </div>
    </header>
  )
}
