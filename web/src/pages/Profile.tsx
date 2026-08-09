import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { Button, Card, Input, Spinner } from '../components/ui'
import { api } from '../api/client'
import { mediaUrl } from '../api/media'
import { errorMessage } from '../api/errors'
import { useSubmitOnce } from '../hooks/useSubmitOnce'
import { useAuth } from '../auth/useAuth'

// 性别可选项：受控下拉，避免自由输入不一致。
const GENDER_OPTIONS = ['男', '女', '其他'] as const

// 头像上传校验：与后端契约一致（png/jpg/webp，≤2MB）。
const MAX_AVATAR_BYTES = 2 * 1024 * 1024
const ACCEPTED_AVATAR = ['image/png', 'image/jpeg', 'image/webp']

type ProfileForm = {
  nickname: string
  age: string
  gender: string
}

export default function Profile() {
  const { user, refreshUser } = useAuth()

  const [form, setForm] = useState<ProfileForm>({
    nickname: user?.nickname ?? '',
    age: user?.age != null ? String(user.age) : '',
    gender: user?.gender ?? '',
  })
  const [savedHint, setSavedHint] = useState('')
  const [error, setError] = useState('')

  const avatarInputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [avatarError, setAvatarError] = useState('')

  const isAdmin = user?.role === 'admin'
  const avatarPreview = mediaUrl(user?.avatarUrl)
  const initial = user?.nickname?.trim()?.[0] ?? '🙂'

  const update = (patch: Partial<ProfileForm>) => setForm((prev) => ({ ...prev, ...patch }))

  const { submit: onSave, submitting: saving } = useSubmitOnce(async () => {
    if (!form.nickname.trim()) {
      setError('先起个昵称吧～')
      return
    }
    // 年龄容错：非法/留空按 0 处理，避免 NaN 提交。
    const age = Number.parseInt(form.age, 10)
    setError('')
    setSavedHint('')
    try {
      await api.updateProfile({
        nickname: form.nickname.trim(),
        age: Number.isNaN(age) ? 0 : age,
        gender: form.gender,
      })
      await refreshUser()
      setSavedHint('已保存 ✨')
    } catch (err) {
      setError(errorMessage(err))
    }
  })

  const handleAvatarFile = async (file: File) => {
    if (!ACCEPTED_AVATAR.includes(file.type)) {
      setAvatarError('只支持 png / jpg / webp 图片哦')
      return
    }
    if (file.size > MAX_AVATAR_BYTES) {
      setAvatarError('图片有点大，换一张 2MB 以内的吧～')
      return
    }
    setAvatarError('')
    setUploading(true)
    try {
      await api.uploadAvatar(file)
      await refreshUser()
    } catch (err) {
      setAvatarError(errorMessage(err))
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader
        right={
          <Link to="/my">
            <Button variant="ghost" className="text-sm">
              ← 返回
            </Button>
          </Link>
        }
      />

      <main className="mx-auto flex max-w-2xl flex-col gap-6 px-6 py-8">
        <h1 className="font-display text-3xl font-extrabold text-ink">个人资料 🧑‍🎨</h1>

        {/* 头像 */}
        <Card className="flex flex-col items-center gap-4">
          <button
            type="button"
            onClick={() => avatarInputRef.current?.click()}
            className="relative flex h-28 w-28 items-center justify-center overflow-hidden rounded-full bg-gradient-to-b from-sun to-peach font-display text-4xl font-bold text-white shadow-soft-sm transition-transform hover:-translate-y-0.5"
            aria-label="上传头像"
          >
            {uploading ? (
              <Spinner size={32} />
            ) : avatarPreview ? (
              <img src={avatarPreview} alt="头像预览" className="h-full w-full object-cover" />
            ) : (
              <span>{initial}</span>
            )}
          </button>
          <button
            type="button"
            onClick={() => avatarInputRef.current?.click()}
            className="text-sm font-semibold text-sky-deep hover:text-coral"
          >
            {avatarPreview ? '换一张头像' : '上传头像'}
          </button>
          <p className="text-xs text-ink-soft/60">点击头像上传 · png/jpg/webp · ≤2MB</p>

          {avatarError && (
            <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
              {avatarError}
            </p>
          )}

          <input
            ref={avatarInputRef}
            type="file"
            accept={ACCEPTED_AVATAR.join(',')}
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0]
              if (file) void handleAvatarFile(file)
              e.target.value = ''
            }}
          />
        </Card>

        {/* 资料表单 */}
        <Card className="flex flex-col gap-5">
          <Input
            id="profile-nickname"
            label="昵称"
            placeholder="给自己起个名字吧～"
            value={form.nickname}
            onChange={(e) => update({ nickname: e.target.value })}
          />

          <div className="grid grid-cols-2 gap-4">
            <Input
              id="profile-age"
              label="年龄"
              type="number"
              min={0}
              placeholder="例如：7"
              value={form.age}
              onChange={(e) => update({ age: e.target.value })}
            />
            <label className="block" htmlFor="profile-gender">
              <span className="mb-1.5 block px-1 text-sm font-semibold text-ink-soft">性别</span>
              <select
                id="profile-gender"
                className="field"
                value={form.gender}
                onChange={(e) => update({ gender: e.target.value })}
              >
                <option value="">未选择</option>
                {GENDER_OPTIONS.map((g) => (
                  <option key={g} value={g}>
                    {g}
                  </option>
                ))}
              </select>
            </label>
          </div>

          {error && (
            <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
              {error}
            </p>
          )}
          {savedHint && (
            <p className="rounded-2xl bg-mint/20 px-4 py-3 text-center text-sm font-semibold text-ink-soft">
              {savedHint}
            </p>
          )}

          <Button onClick={onSave} loading={saving} className="self-end">
            保存 ✨
          </Button>
        </Card>

        {/* 邀请码（仅管理员） */}
        {isAdmin && <InviteCodeCard />}
      </main>
    </div>
  )
}

// 邀请码卡片：仅管理员渲染。进页拉当前邀请码，可轮换。
function InviteCodeCard() {
  const [code, setCode] = useState('')
  const [used, setUsed] = useState(0)
  const [limit, setLimit] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let alive = true
    ;(async () => {
      try {
        const status = await api.getInviteCode()
        if (alive) {
          setCode(status.inviteCode)
          setUsed(status.used)
          setLimit(status.limit)
        }
      } catch (err) {
        if (alive) setError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [])

  const { submit: onRotate, submitting: rotating } = useSubmitOnce(async () => {
    setError('')
    try {
      const status = await api.rotateInviteCode()
      setCode(status.inviteCode)
      setUsed(status.used)
      setLimit(status.limit)
    } catch (err) {
      setError(errorMessage(err))
    }
  })

  // limit 为 0 表示不限制；否则展示"已用 X/上限"，用尽时高亮提示。
  const exhausted = limit > 0 && used >= limit

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <h2 className="font-display text-xl font-extrabold text-ink">邀请码 🎟️</h2>
        <span className="rounded-full bg-sky/20 px-2 py-0.5 text-xs font-semibold text-sky-deep">
          仅管理员
        </span>
      </div>
      <p className="text-sm text-ink-soft/70">
        新用户注册时需要填写此邀请码。轮换后旧码立即失效，名额重新计算。
      </p>

      {loading ? (
        <div className="flex items-center gap-2 text-ink-soft">
          <Spinner size={20} />
          <span className="text-sm">正在读取…</span>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <code className="select-all rounded-2xl bg-cream/80 px-4 py-3 font-mono text-lg font-bold tracking-wider text-ink">
              {code || '—'}
            </code>
            <Button variant="ghost" onClick={onRotate} loading={rotating} className="text-sm">
              🔄 轮换
            </Button>
          </div>
          {limit > 0 ? (
            <span
              className={`inline-flex w-fit items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold ${
                exhausted ? 'bg-coral/15 text-coral' : 'bg-meadow/20 text-meadow-deep'
              }`}
            >
              {exhausted ? '名额已用完' : '名额'} {used}/{limit}
              {exhausted && ' · 轮换可再邀请'}
            </span>
          ) : (
            <span className="inline-flex w-fit items-center rounded-full bg-sky/15 px-3 py-1 text-xs font-semibold text-sky-deep">
              已邀请 {used} 人 · 不限名额
            </span>
          )}
        </div>
      )}

      {error && (
        <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}
    </Card>
  )
}
