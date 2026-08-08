import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { AppHeader } from '../components/AppHeader'
import { Button, EmptyState, LoadingClouds } from '../components/ui'
import { Stepper, type Stage } from '../components/chapter/Stepper'
import { ChatStoryboard } from '../components/ChatStoryboard'
import { PanelGrid } from '../components/PanelGrid'
import { ComicCompose } from '../components/ComicCompose'
import { useChapterData } from '../components/chapter/useChapterData'
import type { Chapter, ChapterStatus, Panel } from '../api/types'

// 章节状态 → 初始步骤。
function stageFromStatus(status: ChapterStatus): Stage {
  if (status === 'draft') return 1
  if (status === 'storyboarding') return 2
  return 3 // rendering | done
}

export default function ChapterEditor() {
  const { id } = useParams<{ id: string }>()
  const chapterId = id ?? ''
  const navigate = useNavigate()

  const { chapter, panels, index, loading, error, setPanels, setChapter } =
    useChapterData(chapterId)

  // 对话式分镜每轮返回全量分镜，直接替换本地状态。
  const replacePanels = (next: Panel[]) => setPanels(next)
  const [stage, setStage] = useState<Stage>(1)

  useEffect(() => {
    if (chapter) setStage(stageFromStatus(chapter.status))
  }, [chapter])

  // 已解锁的最大步骤：随章节进度推进，用户可回退。
  const maxReached: Stage = chapter ? stageFromStatus(chapter.status) : 1

  // Stage ① 确认分镜：分镜已在每轮对话中持久化，这里只推进步骤。
  const confirmStoryboard = () => {
    if (chapter) setChapter({ ...chapter, status: 'storyboarding' })
    setStage(2)
  }

  const onSaved = (ch: Chapter) => {
    setChapter(ch)
    // 封面保存后回到书房工作台；普通章节进入阅读器。
    navigate(ch.isCover ? `/books/${ch.bookId}` : `/read/${chapterId}`)
  }

  return (
    <div className="min-h-screen bg-cream">
      <AppHeader
        right={
          chapter && (
            <Link to={`/books/${chapter.bookId}`}>
              <Button variant="ghost" className="text-sm">
                ← 书房
              </Button>
            </Link>
          )
        }
      />

      <main className="mx-auto max-w-5xl px-6 py-8">
        {loading ? (
          <LoadingClouds label="正在打开故事分镜台…" />
        ) : error || !chapter ? (
          <EmptyState
            emoji="🌧️"
            title="这一章打不开"
            description={error || '找不到这个章节'}
            action={
              <Link to="/">
                <Button variant="ghost">回到书架</Button>
              </Link>
            }
          />
        ) : (
          <>
            <header className="mb-6 text-center">
              <h1 className="font-display text-3xl font-extrabold text-ink">
                {chapter.isCover ? '制作封面 🎨' : chapter.title}
              </h1>
            </header>

            <Stepper current={stage} maxReached={maxReached} onGo={setStage} />

            {stage === 1 && (
              <ChatStoryboard
                chapterId={chapterId}
                panels={panels}
                index={index}
                onPanelsChange={replacePanels}
                onConfirm={confirmStoryboard}
                conversation={chapter.conversation ?? []}
                savedPanelCount={chapter.panelCount ?? 0}
                coverMode={chapter.isCover}
              />
            )}
            {stage === 2 && (
              <PanelGrid
                panels={panels}
                index={index}
                onPanelsChange={setPanels}
                onNext={() => setStage(3)}
              />
            )}
            {stage === 3 && (
              <ComicCompose
                chapterId={chapterId}
                title={chapter.isCover ? '封面' : chapter.title}
                panels={panels}
                onSaved={onSaved}
                coverMode={chapter.isCover}
              />
            )}
          </>
        )}
      </main>
    </div>
  )
}
