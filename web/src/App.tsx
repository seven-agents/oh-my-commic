import { Routes, Route, Navigate } from 'react-router-dom'
import { RequireAuth } from './auth/RequireAuth'
import { AppShell } from './components/shell/AppShell'
import Login from './pages/Login'
import CommunityView from './pages/CommunityView'
import MyBooksView from './pages/MyBooksView'
import BookWorkspace from './pages/BookWorkspace'
import AssetEditor from './pages/AssetEditor'
import ChapterEditor from './pages/ChapterEditor'
import Reader from './pages/Reader'
import BookReader from './pages/BookReader'
import Profile from './pages/Profile'
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

      {/* 应用壳：左栏 + 右侧 Outlet。/、/community、/my 在壳内。 */}
      <Route element={<AppShell />}>
        <Route path="/" element={<Navigate to="/community" replace />} />
        <Route path="/community" element={<CommunityView />} />
        <Route path="/my" element={<Protected><MyBooksView /></Protected>} />
      </Route>

      {/* 全屏公开阅读器（不进壳，保持沉浸） */}
      <Route path="/community/books/:id" element={<CommunityReader />} />

      <Route path="/profile" element={<Protected><Profile /></Protected>} />
      <Route path="/books/:id" element={<Protected><BookWorkspace /></Protected>} />
      <Route path="/books/:id/read" element={<Protected><BookReader /></Protected>} />
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
