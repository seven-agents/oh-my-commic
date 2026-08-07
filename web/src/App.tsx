import { Routes, Route } from 'react-router-dom'
import { RequireAuth } from './auth/RequireAuth'
import Login from './pages/Login'
import Bookshelf from './pages/Bookshelf'
import BookWorkspace from './pages/BookWorkspace'
import AssetEditor from './pages/AssetEditor'
import ChapterDetail from './pages/ChapterDetail'
import Reader from './pages/Reader'
import NotFound from './pages/NotFound'
import type { ReactNode } from 'react'

function Protected({ children }: { children: ReactNode }) {
  return <RequireAuth>{children}</RequireAuth>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />

      <Route path="/" element={<Protected><Bookshelf /></Protected>} />
      <Route path="/books/:id" element={<Protected><BookWorkspace /></Protected>} />

      {/* 资产编辑：新建 & 编辑，kind ∈ character|pet|scene，assetId 为 'new' 或数字 id */}
      <Route
        path="/books/:bookId/assets/:kind/:assetId"
        element={<Protected><AssetEditor /></Protected>}
      />

      <Route path="/chapters/:id" element={<Protected><ChapterDetail /></Protected>} />
      <Route path="/read/:chapterId" element={<Protected><Reader /></Protected>} />

      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}
