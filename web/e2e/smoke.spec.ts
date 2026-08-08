import { expect, test } from '@playwright/test'

// 只覆盖「不触发 AI」的关键路径：注册 → 自动登录落书架 → 建书 → 书架可见 →
// 登出回登录页 → 受保护路由拦截跳登录。绝不进入出图/分镜/漫画化（会调 LLM）。

// 与 CI e2e.yml 注入的 INVITE_CODE 保持一致（假值，仅放行注册）。
const INVITE_CODE = 'welcome123'
// 满足后端密码规则：8-64 位且同时含字母与数字。
const PASSWORD = 'Passw0rd1'

// 每条测试用唯一 username，避免撞已存在用户（409）。
// Playwright 测试跑在 Node，可用 Date.now()。username 需小写字母开头、3-20 位。
function uniqueUsername(): string {
  return `u${Date.now()}${Math.floor(Math.random() * 1000)}`.slice(0, 20)
}

// 在登录页完成注册（username/email/inviteCode），注册成功后前端自动登录并跳到书架。
async function registerAndLand(page: import('@playwright/test').Page, username: string) {
  await page.goto('/login')
  // 切到「注册」Tab
  await page.getByRole('button', { name: '注册' }).click()
  // 用字段 id 定位（Input 的 label 含 hint span，getByLabel 会拿到含提示的复合文案）。
  await page.locator('#username').fill(username)
  await page.locator('#password').fill(PASSWORD)
  await page.locator('#email').fill(`${username}@example.com`)
  await page.locator('#inviteCode').fill(INVITE_CODE)
  await page.getByRole('button', { name: /注册并开始/ }).click()

  // 落到书架
  await expect(page.getByRole('heading', { name: /我的书架/ })).toBeVisible()
}

test('注册新用户后自动登录落到书架', async ({ page }) => {
  await registerAndLand(page, uniqueUsername())
  await expect(page).toHaveURL(/\/$|\/$/)
  await expect(page.getByRole('heading', { name: /我的书架/ })).toBeVisible()
})

test('建一本书后书架可见', async ({ page }) => {
  await registerAndLand(page, uniqueUsername())

  const title = `我的漫画_${Date.now()}`

  // 打开「创作新书」弹窗（新用户空书架有一个入口按钮）
  await page.getByRole('button', { name: /创作新书/ }).first().click()
  await page.getByLabel('书名').fill(title)
  await page.getByRole('button', { name: /开始创作/ }).click()

  // 建书成功后前端跳到该书的工作台，标题应出现（可能出现在多处，取首个）
  await expect(page.getByText(title).first()).toBeVisible()

  // 回到书架，断言这本书出现在书架上（书卡封面 + 标题，取首个即可）
  await page.goto('/')
  await expect(page.getByRole('heading', { name: /我的书架/ })).toBeVisible()
  await expect(page.getByText(title).first()).toBeVisible()
})

test('登出后回到登录页', async ({ page }) => {
  await registerAndLand(page, uniqueUsername())

  // 打开右上角用户菜单 → 退出
  await page.getByRole('button', { name: '用户菜单' }).click()
  await page.getByRole('button', { name: '退出' }).click()

  // 回到登录页
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('heading', { name: 'oh-my-commic' })).toBeVisible()
})

test('未登录直接访问受保护路由被拦截跳登录', async ({ page }) => {
  // 全新上下文，无登录态；访问书架应被 RequireAuth 重定向到 /login
  await page.goto('/')
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByRole('button', { name: /开始画画/ })).toBeVisible()
})
