// Vitest 全局 setup：引入 jest-dom 断言扩展（toBeInTheDocument 等），
// 并在每个用例后清理已渲染的组件，保证测试隔离。
import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

afterEach(() => {
  cleanup()
})
