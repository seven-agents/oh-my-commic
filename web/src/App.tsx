import { Routes, Route } from 'react-router-dom'
import { RequireAuth } from './auth/RequireAuth'
import Login from './pages/Login'
import Bookshelf from './pages/Bookshelf'
import BookDetail from './pages/BookDetail'
import AssetNew from './pages/AssetNew'
import ChapterDetail from './pages/ChapterDetail'
import Reader from './pages/Reader'
import NotFound from './pages/NotFound'

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />

      <Route
        path="/"
        element={
          <RequireAuth>
            <Bookshelf />
          </RequireAuth>
        }
      />
      <Route
        path="/books/:id"
        element={
          <RequireAuth>
            <BookDetail />
          </RequireAuth>
        }
      />
      <Route
        path="/books/:id/assets/new"
        element={
          <RequireAuth>
            <AssetNew />
          </RequireAuth>
        }
      />
      <Route
        path="/chapters/:id"
        element={
          <RequireAuth>
            <ChapterDetail />
          </RequireAuth>
        }
      />
      <Route
        path="/read/:chapterId"
        element={
          <RequireAuth>
            <Reader />
          </RequireAuth>
        }
      />

      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}
