import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { ImageUploadZone } from '../components/ImageUploadZone'
import { Button, Card, Input, LoadingClouds, Textarea } from '../components/ui'
import { api } from '../api/client'
import type { Character, CharacterType, Scene } from '../api/types'
import { errorMessage } from '../api/errors'
import { useSubmitOnce } from '../hooks/useSubmitOnce'
import { useAuth } from '../auth/useAuth'

type AssetKind = 'character' | 'pet' | 'scene'

const KIND_META: Record<AssetKind, { title: string; emoji: string }> = {
  character: { title: '角色', emoji: '🧒' },
  pet: { title: '宠物', emoji: '🐾' },
  scene: { title: '场景', emoji: '🏞️' },
}

type FormState = {
  name: string
  gender: string
  age: string
  personality: string
  description: string
  imageUrl: string
}

const EMPTY_FORM: FormState = { name: '', gender: '', age: '', personality: '', description: '', imageUrl: '' }

export default function AssetEditor() {
  const navigate = useNavigate()
  const { refreshUser } = useAuth()
  const { bookId = '', kind = 'character', assetId } = useParams<{
    bookId: string
    kind: AssetKind
    assetId: string
  }>()

  const isScene = kind === 'scene'
  const isEdit = assetId !== undefined && assetId !== 'new'
  const meta = KIND_META[kind as AssetKind] ?? KIND_META.character

  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [loading, setLoading] = useState(isEdit)
  const [error, setError] = useState('')

  const backTo = useMemo(() => `/books/${bookId}`, [bookId])

  useEffect(() => {
    if (!isEdit) return
    let alive = true
    ;(async () => {
      try {
        if (isScene) {
          const list = await api.get<Scene[]>(`/api/v1/books/${bookId}/scenes`)
          const s = (list ?? []).find((x) => String(x.id) === assetId)
          if (alive && s) setForm({ ...EMPTY_FORM, name: s.name, description: s.description, imageUrl: s.imageUrl })
        } else {
          const list = await api.get<Character[]>(`/api/v1/books/${bookId}/characters`)
          const c = (list ?? []).find((x) => String(x.id) === assetId)
          if (alive && c)
            setForm({
              name: c.name,
              gender: c.gender,
              age: c.age,
              personality: c.personality,
              description: c.description,
              imageUrl: c.imageUrl,
            })
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
  }, [isEdit, isScene, bookId, assetId])

  const update = (patch: Partial<FormState>) => setForm((prev) => ({ ...prev, ...patch }))

  // 仅当编辑已有资产、且当前形象图是本地 /media/ 图时，才允许重新生成。
  const canRegenerate = isEdit && form.imageUrl.startsWith('/media/')

  // 重新生成：对当前锁定形象图再跑一次漫画化，覆盖预览并刷新顶栏积分。
  // 复用同步守卫防双击重复 POST。
  const { submit: onRegenerate, submitting: regenerating } = useSubmitOnce(async () => {
    setError('')
    try {
      const path = `/api/v1/books/${bookId}/${isScene ? 'scenes' : 'characters'}/${assetId}/regenerate`
      const next = isScene ? await api.post<Scene>(path) : await api.post<Character>(path)
      update({ imageUrl: next.imageUrl })
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      // 重画会扣费（失败退还 / 余额不足 402），刷新 header 积分。
      void refreshUser()
    }
  })

  const { submit: onSave, submitting: saving } = useSubmitOnce(async () => {
    if (!form.name.trim()) {
      setError('先起个名字吧～')
      return
    }
    setError('')
    try {
      if (isScene) {
        const body = { name: form.name.trim(), description: form.description, imageUrl: form.imageUrl }
        if (isEdit) await api.put(`/api/v1/books/${bookId}/scenes/${assetId}`, body)
        else await api.post(`/api/v1/books/${bookId}/scenes`, body)
      } else {
        const body = {
          type: kind as CharacterType,
          name: form.name.trim(),
          gender: form.gender,
          age: form.age,
          personality: form.personality,
          description: form.description,
          imageUrl: form.imageUrl,
        }
        if (isEdit) await api.put(`/api/v1/books/${bookId}/characters/${assetId}`, body)
        else await api.post(`/api/v1/books/${bookId}/characters`, body)
      }
      navigate(backTo)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      // 新上传的图会漫画化（扣费；失败退还 / 余额不足 402），保存后刷新 header 积分。
      void refreshUser()
    }
  })

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader
        right={
          <Link to={backTo}>
            <Button variant="ghost" className="text-sm">
              ← 返回
            </Button>
          </Link>
        }
      />

      <main className="mx-auto max-w-2xl px-6 py-8">
        <h1 className="mb-6 font-display text-3xl font-extrabold text-ink">
          {isEdit ? '编辑' : '新建'}
          {meta.title} {meta.emoji}
        </h1>

        {loading ? (
          <LoadingClouds label="正在读取…" />
        ) : saving || regenerating ? (
          <Card className="flex flex-col items-center gap-2 py-10 text-center">
            <LoadingClouds
              label={isScene ? '正在把场景画成绘本背景… ✨' : '正在把 TA 画成漫画角色，稍等一下下～ ✨'}
            />
            <p className="text-sm text-ink-soft/70">AI 正在施展吉卜力魔法，大约需要 15–30 秒</p>
          </Card>
        ) : (
          <Card className="flex flex-col gap-5">
            <ImageUploadZone bookId={bookId} imageUrl={form.imageUrl} onUploaded={(url) => update({ imageUrl: url })} />
            <p className="-mt-2 text-center text-xs text-ink-soft/70">
              {isScene
                ? '保存时 AI 会把这张图画成吉卜力风格的绘本背景 ✨'
                : '保存时 AI 会把这张图画成吉卜力风格的角色形象 ✨'}
            </p>

            {canRegenerate && (
              <Button variant="ghost" onClick={onRegenerate} loading={regenerating} className="-mt-2 self-center text-sm">
                🔄 重新生成
              </Button>
            )}

            <Input
              id="asset-name"
              label="名字"
              placeholder={isScene ? '例如：魔法森林' : '例如：小狐狸阿黄'}
              value={form.name}
              onChange={(e) => update({ name: e.target.value })}
            />

            {!isScene && (
              <>
                <div className="grid grid-cols-2 gap-4">
                  <Input
                    id="asset-gender"
                    label="性别"
                    placeholder="男孩 / 女孩…"
                    value={form.gender}
                    onChange={(e) => update({ gender: e.target.value })}
                  />
                  <Input
                    id="asset-age"
                    label="年龄"
                    placeholder="例如：7 岁"
                    value={form.age}
                    onChange={(e) => update({ age: e.target.value })}
                  />
                </div>
                <Input
                  id="asset-personality"
                  label="性格"
                  placeholder="例如：勇敢又爱笑"
                  value={form.personality}
                  onChange={(e) => update({ personality: e.target.value })}
                />
              </>
            )}

            <Textarea
              id="asset-description"
              label="描述"
              rows={3}
              placeholder={isScene ? '这个地方长什么样？' : '外貌、穿着、喜欢什么…'}
              value={form.description}
              onChange={(e) => update({ description: e.target.value })}
            />

            {error && (
              <p className="rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
                {error}
              </p>
            )}

            <div className="flex gap-3">
              <Link to={backTo} className="flex-1">
                <Button variant="ghost" className="w-full">
                  取消
                </Button>
              </Link>
              <Button onClick={onSave} loading={saving} className="flex-1">
                保存 ✨
              </Button>
            </div>
          </Card>
        )}
      </main>
    </div>
  )
}
