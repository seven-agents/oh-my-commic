import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import Home from './Home'
import { AuthProvider } from '../auth/useAuth'

describe('Home', () => {
  it('展示社区与我的漫画两个入口', () => {
    render(
      <MemoryRouter>
        <AuthProvider>
          <Home />
        </AuthProvider>
      </MemoryRouter>,
    )
    expect(screen.getByRole('link', { name: /社区/ })).toHaveAttribute('href', '/community')
    expect(screen.getByRole('link', { name: /我的漫画/ })).toHaveAttribute('href', '/my')
  })
})
