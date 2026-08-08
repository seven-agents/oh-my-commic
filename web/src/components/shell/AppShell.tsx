import { Outlet } from 'react-router-dom'
import { SideNav } from './SideNav'

// 应用壳：左侧常驻导航 + 右侧主内容区（随路由 Outlet 切换）。
// 桌面左右分栏，窄屏上下堆叠。
export function AppShell() {
  return (
    <div className="flex min-h-screen flex-col bg-cream md:flex-row">
      <SideNav />
      <main className="min-w-0 flex-1">
        <Outlet />
      </main>
    </div>
  )
}
