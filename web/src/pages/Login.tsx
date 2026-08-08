import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Input } from '../components/ui'
import { useAuth } from '../auth/useAuth'
import { ApiError } from '../api/client'
import { useSubmitOnce } from '../hooks/useSubmitOnce'

type Tab = 'login' | 'register'

const MIN_PASSWORD = 6

export default function Login() {
  const navigate = useNavigate()
  const { login, register } = useAuth()

  const [tab, setTab] = useState<Tab>('login')
  const [nickname, setNickname] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  const switchTab = (next: Tab) => {
    setTab(next)
    setError('')
  }

  const validate = (): string | null => {
    if (!nickname.trim()) return '先给自己起个昵称吧～'
    if (password.length < MIN_PASSWORD) return `密码至少 ${MIN_PASSWORD} 位哦`
    return null
  }

  const messageFor = (err: unknown): string => {
    if (err instanceof ApiError) {
      if (err.status === 401) return '昵称或密码不对哦～'
      if (err.status === 409) return '这个昵称被用啦，换一个？'
      if (err.status === 400) return '信息填得不太对，检查一下～'
      return err.message
    }
    return '网络开小差了，待会儿再试试～'
  }

  const { submit, submitting } = useSubmitOnce(async () => {
    setError('')
    const invalid = validate()
    if (invalid) {
      setError(invalid)
      return
    }
    try {
      if (tab === 'login') {
        await login(nickname.trim(), password)
      } else {
        await register(nickname.trim(), password)
      }
      navigate('/', { replace: true })
    } catch (err) {
      setError(messageFor(err))
    }
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    void submit()
  }

  return (
    <div className="relative min-h-screen overflow-hidden bg-gradient-to-b from-sky/40 via-cream to-cream">
      {/* 漂浮的云朵与星星背景 */}
      <div className="pointer-events-none absolute inset-0" aria-hidden>
        <span className="absolute left-[8%] top-[12%] text-6xl animate-float-slow opacity-80">☁️</span>
        <span className="absolute right-[12%] top-[8%] text-7xl animate-float opacity-80">☁️</span>
        <span className="absolute left-[20%] bottom-[14%] text-5xl animate-float opacity-70">☁️</span>
        <span className="absolute right-[16%] bottom-[20%] text-3xl animate-twinkle">⭐️</span>
        <span className="absolute left-[46%] top-[6%] text-2xl animate-twinkle" style={{ animationDelay: '0.8s' }}>
          ✨
        </span>
      </div>

      <div className="relative z-10 flex min-h-screen flex-col items-center justify-center px-4 py-10">
        <header className="mb-8 text-center">
          <div className="mb-3 text-6xl animate-float" aria-hidden>
            🎨
          </div>
          <h1 className="font-display text-5xl font-extrabold tracking-tight text-ink">
            oh-my-commic
          </h1>
          <p className="mt-3 text-lg text-ink-soft">和小朋友一起，画一本自己的漫画书 🎨</p>
        </header>

        <Card className="w-full max-w-md animate-pop-in">
          {/* Tab 切换 */}
          <div className="mb-6 flex rounded-full bg-cream p-1.5">
            <TabButton active={tab === 'login'} onClick={() => switchTab('login')}>
              登录
            </TabButton>
            <TabButton active={tab === 'register'} onClick={() => switchTab('register')}>
              注册
            </TabButton>
          </div>

          <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
            <Input
              id="nickname"
              label="昵称"
              placeholder="给自己起个名字"
              autoComplete="username"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
            />
            <Input
              id="password"
              type="password"
              label="密码"
              placeholder="至少 6 位"
              autoComplete={tab === 'login' ? 'current-password' : 'new-password'}
              hint={tab === 'register' ? '密码至少 6 位，记牢一点哦～' : undefined}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />

            {error && (
              <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
                {error}
              </p>
            )}

            <Button type="submit" loading={submitting} className="mt-2 w-full text-lg">
              {tab === 'login' ? '开始画画 🖍️' : '注册并开始 ✨'}
            </Button>
          </form>
        </Card>

        <p className="mt-6 text-sm text-ink-soft/70">用想象力，画一个属于你的小世界 🌈</p>
      </div>
    </div>
  )
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex-1 rounded-full py-2.5 font-display font-semibold transition-all duration-200 ${
        active ? 'bg-white text-ink shadow-soft-sm' : 'text-ink-soft hover:text-ink'
      }`}
    >
      {children}
    </button>
  )
}
