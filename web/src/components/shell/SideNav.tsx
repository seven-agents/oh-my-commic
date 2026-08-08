import { NavLink, Link } from 'react-router-dom'
import type { ReactNode } from 'react'
import { SideNavUser } from './SideNavUser'

function Tab({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        [
          'flex items-center gap-2 rounded-2xl px-4 py-3 font-display font-bold transition-colors',
          isActive ? 'bg-white text-ink shadow-soft-sm' : 'text-ink-soft hover:bg-white/50',
        ].join(' ')
      }
    >
      {children}
    </NavLink>
  )
}

// 左栏：Logo + 两个 tab + 底部用户区。桌面竖排(md:)，窄屏收成顶部横排。
export function SideNav() {
  return (
    <nav className="flex shrink-0 flex-row items-center gap-3 border-b border-ink/5 bg-cream/80 px-4 py-3 backdrop-blur md:h-screen md:w-[230px] md:flex-col md:items-stretch md:border-b-0 md:border-r md:py-6">
      <Link to="/community" className="font-display text-xl font-extrabold text-ink md:mb-4 md:px-2">
        🎨 oh-my-commic
      </Link>
      <div className="flex flex-1 flex-row gap-2 md:flex-col md:flex-none">
        <Tab to="/community">🌈 社区</Tab>
        <Tab to="/my">📚 我的漫画</Tab>
      </div>
      <div className="md:mt-auto">
        <SideNavUser />
      </div>
    </nav>
  )
}
