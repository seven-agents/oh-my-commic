// 三步走导航条：① 讲故事 ② 画分镜 ③ 拼成书。
// 已完成的步骤可点击回退；未解锁的步骤不可点。

export type Stage = 1 | 2 | 3

type Step = { stage: Stage; emoji: string; label: string }

const STEPS: Step[] = [
  { stage: 1, emoji: '💬', label: '讲故事' },
  { stage: 2, emoji: '🎬', label: '画分镜' },
  { stage: 3, emoji: '📚', label: '拼成书' },
]

type StepperProps = {
  current: Stage
  // 已解锁（可点击回退）的最大步骤
  maxReached: Stage
  onGo: (stage: Stage) => void
}

export function Stepper({ current, maxReached, onGo }: StepperProps) {
  return (
    <nav className="mx-auto mb-8 flex max-w-xl items-center justify-center gap-1 sm:gap-3">
      {STEPS.map((step, i) => {
        const active = step.stage === current
        const reachable = step.stage <= maxReached
        return (
          <div key={step.stage} className="flex items-center gap-1 sm:gap-3">
            <button
              type="button"
              disabled={!reachable}
              onClick={() => reachable && onGo(step.stage)}
              className={[
                'flex items-center gap-2 rounded-full px-4 py-2.5 font-display font-bold transition-all',
                active
                  ? 'bg-gradient-to-b from-peach to-coral text-white shadow-soft'
                  : reachable
                    ? 'bg-white text-ink shadow-soft-sm hover:-translate-y-0.5'
                    : 'cursor-not-allowed bg-white/50 text-ink-soft/50',
              ].join(' ')}
            >
              <span
                className={[
                  'flex h-7 w-7 flex-none items-center justify-center rounded-full text-sm',
                  active ? 'bg-white/25' : 'bg-cream',
                ].join(' ')}
                aria-hidden
              >
                {step.stage}
              </span>
              <span className="hidden sm:inline">
                {step.emoji} {step.label}
              </span>
              <span className="sm:hidden" aria-hidden>
                {step.emoji}
              </span>
            </button>
            {i < STEPS.length - 1 && (
              <span className="text-lg text-ink-soft/40" aria-hidden>
                →
              </span>
            )}
          </div>
        )
      })}
    </nav>
  )
}
