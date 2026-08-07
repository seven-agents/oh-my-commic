import { Link } from 'react-router-dom'
import { Button, EmptyState } from '../components/ui'

export default function NotFound() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-cream px-4">
      <EmptyState
        emoji="🧭"
        title="咦，这里没有页面"
        description="也许翻错了一页。我们一起回到书架吧～"
        action={
          <Link to="/">
            <Button>回到书架</Button>
          </Link>
        }
      />
    </div>
  )
}
