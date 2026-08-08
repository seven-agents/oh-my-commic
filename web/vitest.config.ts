import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// 前端单元测试配置。只测纯逻辑与 hook，不碰网络、不 mock 后端。
// e2e 目录交给 Playwright，这里显式排除，避免 Vitest 误收。
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
    exclude: ['node_modules', 'dist', 'e2e'],
  },
})
