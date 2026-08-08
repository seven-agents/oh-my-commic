import type { CommunitySort } from '../../api/types'

const OPTIONS: { value: CommunitySort; label: string }[] = [
  { value: 'new', label: '最新' },
  { value: 'hot', label: '最热' },
]

// 最新/最热分段切换控件。
export function SortToggle({
  value,
  onChange,
}: {
  value: CommunitySort
  onChange: (v: CommunitySort) => void
}) {
  return (
    <div className="inline-flex rounded-full bg-white/70 p-1 shadow-soft-sm">
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
          className={[
            'rounded-full px-4 py-1.5 font-display text-sm font-bold transition-colors',
            value === o.value ? 'bg-gradient-to-b from-sun to-peach text-white' : 'text-ink-soft',
          ].join(' ')}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
