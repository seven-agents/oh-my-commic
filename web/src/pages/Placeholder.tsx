import { Link, useParams } from 'react-router-dom'
import { EmptyState, Button } from '../components/ui'
import { useAuth } from '../auth/useAuth'

// 占位页面：真实页面将在后续实现。
type PlaceholderProps = {
  emoji: string
  title: string
  description: string
}

export function Placeholder({ emoji, title, description }: PlaceholderProps) {
  const params = useParams()
  const { logout } = useAuth()
  const ids = Object.entries(params)

  return (
    <div className="min-h-screen bg-cream">
      <header className="flex items-center justify-between px-6 py-4">
        <Link to="/" className="font-display text-2xl font-extrabold text-ink">
          🎨 oh-my-commic
        </Link>
        <Button variant="ghost" onClick={() => logout()} className="text-sm">
          退出
        </Button>
      </header>

      <main className="mx-auto max-w-3xl px-6 py-16">
        <EmptyState
          emoji={emoji}
          title={title}
          description={description}
          action={
            <Link to="/">
              <Button variant="ghost">回到书架</Button>
            </Link>
          }
        />
        {ids.length > 0 && (
          <p className="mt-6 text-center text-sm text-ink-soft/60">
            {ids.map(([k, v]) => `${k}: ${v}`).join(' · ')}
          </p>
        )}
      </main>
    </div>
  )
}
