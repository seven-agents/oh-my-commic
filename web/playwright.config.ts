import { defineConfig, devices } from '@playwright/test'

// E2E 只覆盖「不触发 AI」的关键路径（注册/登录/建书/登出/受保护路由拦截）。
// Go 服务由 CI 的 workflow 步骤单独启动（假 key + 临时 DB），这里不用 webServer，
// 避免在 CI 里重复起进程 / 端口冲突。本地调试想让 Playwright 自己起服务可自行加 webServer。
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 1,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { open: 'never' }], ['list']],
  use: {
    baseURL: 'http://127.0.0.1:8080',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
