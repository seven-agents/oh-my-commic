import { Link } from 'react-router-dom'
import { useAuth } from '../../auth/useAuth'
import { mediaUrl } from '../../api/media'

// 左栏底部用户区：登录态显示头像/昵称/积分/资料/退出；未登录显示登录·注册入口。
export function SideNavUser() {
  const { user, isAuthed, logout } = useAuth()

  if (!isAuthed || !user) {
    return (
      <Link
        to="/login"
        className="block rounded-2xl bg-gradient-to-b from-sun to-peach px-4 py-3 text-center font-display font-bold text-white shadow-soft-sm transition-transform hover:-translate-y-0.5"
      >
        登录 / 注册
      </Link>
    )
  }

  const initial = user.nickname?.trim()?.[0] ?? '🙂'
  const avatar = mediaUrl(user.avatarUrl)

  return (
    <div className="flex flex-col gap-3 rounded-2xl bg-white/60 p-3">
      <div className="flex items-center gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-b from-sun to-peach font-display font-bold text-white">
          {avatar ? <img src={avatar} alt="头像" className="h-full w-full object-cover" /> : initial}
        </span>
        <div className="min-w-0">
          <p className="truncate font-display font-bold text-ink">{user.nickname}</p>
          <p className="text-xs font-semibold text-ink-soft" title="图像积分：出图/漫画化各扣 1 分，失败退还">
            ⭐ 积分 {user.credits}
          </p>
        </div>
      </div>
      <div className="flex gap-2">
        <Link
          to="/profile"
          className="flex-1 rounded-xl px-2 py-1.5 text-center font-display text-sm font-semibold text-ink-soft hover:bg-sky/10"
        >
          个人资料
        </Link>
        <button
          type="button"
          onClick={() => logout()}
          className="flex-1 rounded-xl px-2 py-1.5 text-center font-display text-sm font-semibold text-coral hover:bg-coral/10"
        >
          退出
        </button>
      </div>
    </div>
  )
}
