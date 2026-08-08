import { useRef, useState, type DragEvent } from 'react'
import { Spinner } from './ui'
import { api } from '../api/client'
import { mediaUrl } from '../api/media'
import { errorMessage } from '../api/errors'

type ImageUploadZoneProps = {
  bookId: string
  imageUrl: string
  onUploaded: (imageUrl: string) => void
}

const MAX_BYTES = 5 * 1024 * 1024
const ACCEPTED = ['image/png', 'image/jpeg', 'image/webp']

// 上传参考图：拖拽或点击。上传后即锁定为 AI 分镜的一致性参考图。
export function ImageUploadZone({ bookId, imageUrl, onUploaded }: ImageUploadZoneProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const [dragOver, setDragOver] = useState(false)
  const preview = mediaUrl(imageUrl)

  const validate = (file: File): string | null => {
    if (!ACCEPTED.includes(file.type)) return '只支持 png / jpg / webp 图片哦'
    if (file.size > MAX_BYTES) return '图片有点大，换一张 5MB 以内的吧～'
    return null
  }

  const handleFile = async (file: File) => {
    const invalid = validate(file)
    if (invalid) {
      setError(invalid)
      return
    }
    setError('')
    setUploading(true)
    try {
      const { imageUrl: url } = await api.upload<{ imageUrl: string }>(
        `/api/v1/books/${bookId}/upload`,
        file,
      )
      onUploaded(url)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setUploading(false)
    }
  }

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragOver(false)
    const file = e.dataTransfer.files?.[0]
    if (file) void handleFile(file)
  }

  return (
    <div>
      <div
        onClick={() => inputRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault()
          setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={onDrop}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') inputRef.current?.click()
        }}
        className={`flex aspect-video w-full cursor-pointer flex-col items-center justify-center gap-2 overflow-hidden rounded-4xl border-2 border-dashed p-4 text-center transition-all ${
          dragOver ? 'border-coral bg-coral/5' : 'border-ink/15 bg-cream/60 hover:border-sky'
        }`}
      >
        {uploading ? (
          <>
            <Spinner size={32} />
            <p className="font-display text-ink-soft">正在上传…</p>
          </>
        ) : preview ? (
          <img src={preview} alt="参考图预览" className="max-h-full max-w-full rounded-3xl object-contain" />
        ) : (
          <>
            <span className="text-5xl" aria-hidden>
              🖼️
            </span>
            <p className="font-display font-semibold text-ink">上传一张图片</p>
            <p className="max-w-xs text-sm text-ink-soft">
              AI 画分镜时会照着它来保持一致 ✨
            </p>
            <p className="text-xs text-ink-soft/60">点击或拖拽 · png/jpg/webp · ≤5MB</p>
          </>
        )}
      </div>

      {preview && !uploading && (
        <button
          type="button"
          onClick={() => inputRef.current?.click()}
          className="mt-2 px-1 text-sm font-semibold text-sky-deep hover:text-coral"
        >
          换一张图片
        </button>
      )}

      {error && (
        <p className="mt-2 rounded-2xl bg-coral/10 px-4 py-3 text-center text-sm font-semibold text-coral">
          {error}
        </p>
      )}

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPTED.join(',')}
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file) void handleFile(file)
          e.target.value = ''
        }}
      />
    </div>
  )
}
