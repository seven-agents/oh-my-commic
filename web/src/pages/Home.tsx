import { Link } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { Card } from '../components/ui'

// 入口卡：一个大图标 + 标题 + 副标题，整卡可点跳转到目标路由。
type EntryCardProps = {
  to: string
  emoji: string
  title: string
  subtitle: string
}

function EntryCard({ to, emoji, title, subtitle }: EntryCardProps) {
  return (
    <Link to={to} className="block">
      <Card className="flex flex-col items-center gap-3 p-8 text-center transition-transform hover:-translate-y-1">
        <span className="text-6xl" aria-hidden>
          {emoji}
        </span>
        <h2 className="font-display text-2xl font-extrabold text-ink">{title}</h2>
        <p className="text-ink-soft">{subtitle}</p>
      </Card>
    </Link>
  )
}

// 公开落地页：两个大入口（社区 / 我的漫画）。
// 未登录也可点「我的漫画」，进入 /my 由 RequireAuth 引导登录。
export default function Home() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-sky/40 via-cream to-cream">
      <AppHeader />

      <main className="mx-auto max-w-4xl px-4 py-12">
        <header className="mb-10 text-center">
          <div className="mb-3 text-6xl animate-float" aria-hidden>
            🎨
          </div>
          <h1 className="font-display text-4xl font-extrabold tracking-tight text-ink">
            oh-my-commic
          </h1>
          <p className="mt-3 text-lg text-ink-soft">和小朋友一起，画一本自己的漫画书 🌈</p>
        </header>

        <div className="grid gap-6 sm:grid-cols-2">
          <EntryCard
            to="/community"
            emoji="🌟"
            title="社区"
            subtitle="看看大家的漫画"
          />
          <EntryCard
            to="/my"
            emoji="📚"
            title="我的漫画"
            subtitle="创作与管理我的书"
          />
        </div>
      </main>
    </div>
  )
}
