// 社区欢迎横幅：暖色渐变 + slogan，把内容区顶部填满。
export function HeroBanner() {
  return (
    <section className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-sun/30 via-peach/25 to-sky/20 px-8 py-10">
      <div className="relative z-10 max-w-2xl">
        <h1 className="font-display text-3xl font-extrabold text-ink sm:text-4xl">
          和小朋友一起，读一本别人画的魔法漫画 🌈
        </h1>
        <p className="mt-3 font-body text-ink-soft">
          这里是大家公开的绘本作品，翻一翻，说不定就遇见今晚的睡前故事。
        </p>
      </div>
      <span className="pointer-events-none absolute -right-2 top-2 text-6xl opacity-40" aria-hidden>☁️</span>
      <span className="pointer-events-none absolute bottom-2 right-16 text-3xl opacity-40" aria-hidden>⭐</span>
    </section>
  )
}
