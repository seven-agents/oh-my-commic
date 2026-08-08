import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Card, Input } from '../components/ui'
import { useAuth } from '../auth/useAuth'
import { errorMessage } from '../api/errors'
import { useSubmitOnce } from '../hooks/useSubmitOnce'

type Tab = 'login' | 'register'

// 前端即时提示用，与后端保持一致的规则。最终校验仍以后端 400/409 文案为准。
const USERNAME_RE = /^[a-z][a-z0-9_]{2,19}$/
const PASSWORD_LETTER_RE = /[a-zA-Z]/
const PASSWORD_DIGIT_RE = /[0-9]/
const PASSWORD_MIN = 8
const PASSWORD_MAX = 64

// 用户名友好提示：小写字母开头，3-20 位，仅小写字母/数字/下划线。
function usernameHint(username: string): string | null {
  if (!username) return '用户名不能为空哦～'
  if (!USERNAME_RE.test(username)) {
    return '用户名要用小写字母开头，3-20 位，只能有小写字母、数字和下划线～'
  }
  return null
}

// 密码友好提示：8-64 位，且同时包含字母和数字。
function passwordHint(password: string): string | null {
  if (password.length < PASSWORD_MIN || password.length > PASSWORD_MAX) {
    return `密码要 ${PASSWORD_MIN}-${PASSWORD_MAX} 位哦～`
  }
  if (!PASSWORD_LETTER_RE.test(password) || !PASSWORD_DIGIT_RE.test(password)) {
    return '密码要同时有字母和数字，安全一点～'
  }
  return null
}

export default function Login() {
  const navigate = useNavigate()
  const { login, register } = useAuth()

  const [tab, setTab] = useState<Tab>('login')
  const [error, setError] = useState('')

  // 登录字段
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  // 注册字段（含邮箱、邀请码、选填昵称）
  const [email, setEmail] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [nickname, setNickname] = useState('')

  const switchTab = (next: Tab) => {
    setTab(next)
    setError('')
  }

  // 注册前的即时校验：用户名与密码用与后端一致的规则提前拦一道，
  // 邮箱/邀请码只做非空提示，格式与有效性交给后端判定。
  const validateRegister = (): string | null => {
    const uname = usernameHint(username.trim())
    if (uname) return uname
    const pwd = passwordHint(password)
    if (pwd) return pwd
    if (!email.trim()) return '填一下邮箱吧～'
    if (!inviteCode.trim()) return '还需要一个邀请码才能加入哦～'
    return null
  }

  const doLogin = useSubmitOnce(async () => {
    setError('')
    if (!username.trim()) {
      setError('用户名不能为空哦～')
      return
    }
    if (!password) {
      setError('密码不能为空哦～')
      return
    }
    try {
      await login(username.trim(), password)
      navigate('/', { replace: true })
    } catch (err) {
      setError(errorMessage(err))
    }
  })

  const doRegister = useSubmitOnce(async () => {
    setError('')
    const invalid = validateRegister()
    if (invalid) {
      setError(invalid)
      return
    }
    try {
      await register({
        username: username.trim(),
        password,
        email: email.trim(),
        inviteCode: inviteCode.trim(),
        nickname: nickname.trim() || undefined,
      })
      navigate('/', { replace: true })
    } catch (err) {
      setError(errorMessage(err))
    }
  })

  const submitting = tab === 'login' ? doLogin.submitting : doRegister.submitting

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (tab === 'login') {
      void doLogin.submit()
    } else {
      void doRegister.submit()
    }
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
              id="username"
              label="用户名"
              placeholder="小写字母开头，3-20 位"
              autoComplete="username"
              hint={tab === 'register' ? '只能用小写字母、数字和下划线，字母开头～' : undefined}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <Input
              id="password"
              type="password"
              label="密码"
              placeholder={tab === 'register' ? '8-64 位，含字母和数字' : '请输入密码'}
              autoComplete={tab === 'login' ? 'current-password' : 'new-password'}
              hint={tab === 'register' ? '至少 8 位，字母和数字都要有哦～' : undefined}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />

            {tab === 'register' && (
              <>
                <Input
                  id="email"
                  type="email"
                  label="邮箱"
                  placeholder="爸爸妈妈的邮箱"
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
                <Input
                  id="inviteCode"
                  label="邀请码"
                  placeholder="填写收到的邀请码"
                  autoComplete="off"
                  value={inviteCode}
                  onChange={(e) => setInviteCode(e.target.value)}
                />
                <Input
                  id="nickname"
                  label="昵称（选填）"
                  placeholder="想让大家怎么叫你？"
                  autoComplete="nickname"
                  hint="不填的话，就用用户名当昵称啦～"
                  value={nickname}
                  onChange={(e) => setNickname(e.target.value)}
                />
              </>
            )}

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
