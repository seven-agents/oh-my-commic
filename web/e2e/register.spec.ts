import { expect, test } from '@playwright/test'

// 用户管理关键路径 E2E：邀请码注册 → 自动登录落书架 → 进 /profile 改昵称 →
// 断言 header 用户菜单显示新昵称。全程不触发 AI / 邮件。

// 与 CI e2e.yml 注入的 INVITE_CODE 保持一致（假值，仅放行注册）。
const INVITE_CODE = 'welcome123'
// 满足后端密码规则：8-64 位且同时含字母与数字。
const PASSWORD = 'Passw0rd1'

// 唯一 username：小写字母开头、3-20 位、仅小写字母/数字/下划线。
function uniqueUsername(): string {
  return `u${Date.now()}${Math.floor(Math.random() * 1000)}`.slice(0, 20)
}

test('邀请码注册 → 登录落书架 → 改昵称 → header 显示新昵称', async ({ page }) => {
  const username = uniqueUsername()
  const initialNickname = `娃娃_${username}`
  const newNickname = `新昵称_${Date.now()}`

  // 1) 注册：填 username / 密码 / 邮箱 / 邀请码 / 昵称，提交后自动登录。
  await page.goto('/login')
  await page.getByRole('button', { name: '注册' }).click()
  // 用字段 id 定位（Input 的 label 含 hint span，getByLabel 会拿到含提示的复合文案）。
  await page.locator('#username').fill(username)
  await page.locator('#password').fill(PASSWORD)
  await page.locator('#email').fill(`${username}@example.com`)
  await page.locator('#inviteCode').fill(INVITE_CODE)
  await page.locator('#nickname').fill(initialNickname)
  await page.getByRole('button', { name: /注册并开始/ }).click()

  // 2) 落到书架。
  await expect(page.getByRole('heading', { name: /我的书架/ })).toBeVisible()

  // header 用户菜单里应显示注册时填的昵称。
  await page.getByRole('button', { name: '用户菜单' }).click()
  await expect(page.getByText(`你好，${initialNickname}`)).toBeVisible()
  // 通过菜单进入个人资料页。
  await page.getByRole('link', { name: '个人资料' }).click()

  // 3) 在 /profile 改昵称并保存。
  await expect(page).toHaveURL(/\/profile$/)
  const nicknameInput = page.locator('#profile-nickname')
  await nicknameInput.fill(newNickname)
  await page.getByRole('button', { name: /保存/ }).click()
  await expect(page.getByText(/已保存/)).toBeVisible()

  // 4) 断言 header 用户菜单显示新昵称（refreshUser 后 auth 状态已更新）。
  await page.getByRole('button', { name: '用户菜单' }).click()
  await expect(page.getByText(`你好，${newNickname}`)).toBeVisible()
})
