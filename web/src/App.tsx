import { Routes, Route } from 'react-router-dom'
import { RequireAuth } from './auth/RequireAuth'
import Home from './pages/Home'
import Login from './pages/Login'
import Bookshelf from './pages/Bookshelf'
import BookWorkspace from './pages/BookWorkspace'
import AssetEditor from './pages/AssetEditor'
import ChapterEditor from './pages/ChapterEditor'
import Reader from './pages/Reader'
import BookReader from './pages/BookReader'
import Profile from './pages/Profile'
import Community from './pages/Community'
import CommunityReader from './pages/CommunityReader'
import NotFound from './pages/NotFound'
import type { ReactNode } from 'react'

function Protected({ children }: { children: ReactNode }) {
  return <RequireAuth>{children}</RequireAuth>
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />

      <Route path="/" element={<Home />} />
      <Route path="/community" element={<Community />} />
      <Route path="/community/books/:id" element={<CommunityReader />} />
      <Route path="/my" element={<Protected><Bookshelf /></Protected>} />
      <Route path="/profile" element={<Protected><Profile /></Protected>} />
      <Route path="/books/:id" element={<Protected><BookWorkspace /></Protected>} />
      <Route path="/books/:id/read" element={<Protected><BookReader /></Protected>} />

      {/* 资产编辑：新建 & 编辑，kind ∈ character|pet|scene，assetId 为 'new' 或数字 id */}
      <Route
        path="/books/:bookId/assets/:kind/:assetId"
        element={<Protected><AssetEditor /></Protected>}
      />

      <Route path="/chapters/:id" element={<Protected><ChapterEditor /></Protected>} />
      <Route path="/read/:chapterId" element={<Protected><Reader /></Protected>} />

      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}
