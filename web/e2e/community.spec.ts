import { expect, test } from '@playwright/test'

// 社区功能 E2E（全程不触发 AI）：
//  A) 匿名可访问 /community（未登录不被重定向到 /login）。
//  B) owner 注册登录 → 在 /my 建一本书（仅标题，不碰 AI）→ 设为公开 → 该书标题出现在 /community。

// 与 CI e2e.yml 注入的 INVITE_CODE 保持一致（假值，仅放行注册）。
const INVITE_CODE = 'welcome123'
// 满足后端密码规则：8-64 位且同时含字母与数字。
const PASSWORD = 'Passw0rd1'

// 唯一 username：小写字母开头、3-20 位、仅小写字母/数字/下划线。
function uniqueUsername(): string {
  return `u${Date.now()}${Math.floor(Math.random() * 1000)}`.slice(0, 20)
}

// 复用 register.spec 的邀请码注册流程：填字段 → 注册并开始 → 自动登录落 /my（我的书架）。
async function registerAndLand(page: import('@playwright/test').Page, username: string) {
  await page.goto('/login')
  await page.getByRole('button', { name: '注册' }).click()
  // 用字段 id 定位（Input 的 label 含 hint span，getByLabel 会拿到含提示的复合文案）。
  await page.locator('#username').fill(username)
  await page.locator('#password').fill(PASSWORD)
  await page.locator('#email').fill(`${username}@example.com`)
  await page.locator('#inviteCode').fill(INVITE_CODE)
  await page.getByRole('button', { name: /注册并开始/ }).click()

  // 落到书架（/my）。
  await expect(page.getByRole('heading', { name: /我的书架/ })).toBeVisible()
}

test('匿名可访问社区路由，壳与社区内容正常渲染', async ({ page }) => {
  await page.goto('/community')

  // 未登录不应被重定向到 /login。
  await expect(page).toHaveURL(/\/community/)

  // 应用壳左栏「社区」tab 存在（NavLink）。用精确名避免匹配到卡片里含「社区」的书名。
  await expect(page.getByRole('link', { name: '🌈 社区' })).toBeVisible()

  // 社区内容区 hero slogan 可见。
  await expect(page.getByRole('heading', { name: /读一本别人画的魔法漫画/ })).toBeVisible()
})

test('owner 建书并设为公开后，书出现在社区', async ({ page }) => {
  await registerAndLand(page, uniqueUsername())

  const title = `社区漫画_${Date.now()}`

  // 在书架建一本书（空书架有「创作新书」入口按钮）。仅填标题，不触发 AI。
  await page.getByRole('button', { name: /创作新书/ }).first().click()
  await page.getByLabel('书名').fill(title)
  await page.getByRole('button', { name: /开始创作/ }).click()

  // 建书成功后跳到该书工作台，标题应出现。
  await expect(page.getByText(title).first()).toBeVisible()

  // 回到书架，点该书卡的「设为公开」开关。
  await page.goto('/my')
  await expect(page.getByRole('heading', { name: /我的书架/ })).toBeVisible()
  await expect(page.getByText(title).first()).toBeVisible()
  await page.getByRole('button', { name: /设为公开/ }).first().click()
  // 开关切换后文案变为「公开中」，作为发布成功的确认。
  await expect(page.getByRole('button', { name: /公开中/ }).first()).toBeVisible()

  // 打开社区页，断言这本书的标题出现在公开列表里。
  await page.goto('/community')
  await expect(page).toHaveURL(/\/community/)
  await expect(page.getByText(title).first()).toBeVisible()
})
