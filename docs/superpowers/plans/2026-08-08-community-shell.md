# 社区应用壳重设计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把「两卡主页」重构为统一应用壳（左栏 Logo+社区/我的漫画 tab+用户区，右侧随 tab 切换），并把社区内容区做充实（欢迎横幅 + 精选最热3 + 最新/最热排序 + 卡片网格 + 空态），后端 feed 加 `sort` 白名单参数。

**Architecture:** 前端引入 `AppShell` 布局路由（左栏 `SideNav` + 右侧 `<Outlet/>`），`/community` 与 `/my` 作为其子路由，`/` 重定向到 `/community`，公开阅读器 `/community/books/:id` 保持全屏独立。社区页 body 演化为 `CommunityView`、书架 body 演化为 `MyBooksView`（都去掉自带 AppHeader）。后端 `ListPublic` 加 `sort` 入参，白名单映射固定 ORDER BY。

**Tech Stack:** Go(chi, modernc sqlite)、React+Vite+TS+Tailwind+react-router-dom、kin-openapi 契约 E2E、Vitest、Playwright。

## Global Constraints

- **UI 以 spec 结构图为准**（用户不看前端代码，用页面结构图对齐）；沿用现有吉卜力暖色调、`components/ui` 与既有 className 风格。
- **排序安全**：`sort` 走**白名单**映射到固定 ORDER BY 子句；未知/缺省回落 `new`；**绝不把 `sort` 值拼进 SQL**。
- **公开只读隔离不变**：`is_public=1` 过滤 + 非公开/不存在 404；作者只暴露 nickname/avatarUrl；公开详情不含 conversation/panelCount。
- **不可变**：service 返回新对象；React setState 用不可变更新。
- **小文件、单一职责**（200–400 行常态）；**无 console.log**；TS 严格、导出 API 显式类型。
- **API 契约单一真相 = `docs/openapi.yaml`**：改任何 `/api/v1/*` 必须同步 + `test/contract` 覆盖；前端 `web/src/api` 以之为准。
- **测试全 mock**：不碰真实 AI/key；Go 用 `db.Open(":memory:")`+`t.Cleanup`；前端 Vitest mock `../api/client`（注意 mock 需含 `setUnauthorizedHandler`，因 `AuthProvider` 挂载引用它）。
- **git**：分支 `feat/community-shell`（已建）；提交信息中文 `type: 描述`，**不要**署名。

---

## 文件结构

**后端**
- `internal/community/repo.go`（改）：`ListPublic` 加 `sort` 入参 + `orderClause` 白名单。
- `internal/community/service.go`（改）：`ListPublic` 透传 `sort`。
- `internal/community/handler.go`（改）：`List` 读 `?sort`。
- `internal/community/repo_read_test.go`（改）：更新既有 `ListPublic` 调用签名 + 加 `sort=hot` 排序用例。
- `docs/openapi.yaml`（改）：`GET /community/books` 加 `sort` query 参。
- `test/contract/contract_test.go`（改）：加 `?sort=hot` 校验。

**前端（新增/改造）**
- `web/src/api/types.ts`（改）：加 `CommunitySort = 'new'|'hot'`。
- `web/src/api/client.ts`（改）：`listCommunity` 改为 options 入参 `{sort?,limit?,offset?}`。
- `web/src/components/shell/SideNavUser.tsx`（新）：左栏用户区。
- `web/src/components/shell/SideNav.tsx`（新）：左栏（Logo+tabs+用户区）。
- `web/src/components/shell/AppShell.tsx`（新）：布局（SideNav + `<Outlet/>`）。
- `web/src/components/community/HeroBanner.tsx`（新）· `SortToggle.tsx`（新）· `FeaturedRow.tsx`（新）。
- `web/src/pages/CommunityView.tsx`（新）：社区内容区（取代 `Community.tsx`）。
- `web/src/pages/MyBooksView.tsx`（新）：书架内容（取代 `Bookshelf.tsx` 的整页）。
- `web/src/App.tsx`（改）：嵌套路由接壳。
- **删除**：`web/src/pages/Home.tsx`+`Home.test.tsx`、`web/src/pages/Community.tsx`、`web/src/pages/Bookshelf.tsx`+`Bookshelf.test.tsx`（Task 9）。
- `web/e2e/community.spec.ts` / `smoke.spec.ts`（改，Task 10）。
- 文档三处（Task 11）。

---

## Task 1: 后端 feed `sort` 白名单参数

**Files:**
- Modify: `internal/community/repo.go`（`ListPublic` + 新增 `orderClause`）
- Modify: `internal/community/service.go`（`ListPublic` 透传）
- Modify: `internal/community/handler.go`（`List` 读 `?sort`）
- Modify: `internal/community/repo_read_test.go`（更新调用 + 加排序用例）

**Interfaces:**
- Consumes: `Repo`、`CommunityBook`、`likeUserID`。
- Produces:
  - `func (r *Repo) ListPublic(viewerKey, sort string, limit, offset int) ([]CommunityBook, error)`（**签名新增 `sort`**，位于 viewerKey 之后）。
  - `func (s *Service) ListPublic(viewerKey, sort string, limit, offset int) ([]CommunityBook, error)`。
  - `sort` 白名单：`"hot"` → `ORDER BY b.like_count DESC, b.published_at DESC, b.id DESC`；其它（含 `"new"`/`""`/非法）→ `ORDER BY b.published_at DESC, b.id DESC`。

- [ ] **Step 1: 写失败测试** — 在 `internal/community/repo_read_test.go` 追加

```go
func TestListPublicSortHot(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	// A 最新发布但 0 赞；B 较早发布但 5 赞。
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at,like_count) VALUES
	  (10,1,'A新但冷',1,'2026-08-08 12:00:00',0),
	  (11,1,'B旧但热',1,'2026-08-08 10:00:00',5)`)
	repo := NewRepo(d)

	// sort=new：按 published_at 降序 → A(10) 在前。
	byNew, err := repo.ListPublic("", "new", 20, 0)
	if err != nil || len(byNew) != 2 || byNew[0].ID != 10 {
		t.Fatalf("sort=new want [10,11], got %+v err=%v", ids(byNew), err)
	}
	// sort=hot：按 like_count 降序 → B(11) 在前。
	byHot, _ := repo.ListPublic("", "hot", 20, 0)
	if len(byHot) != 2 || byHot[0].ID != 11 {
		t.Fatalf("sort=hot want [11,10], got %+v", ids(byHot))
	}
	// 未知 sort 回落 new。
	byBad, _ := repo.ListPublic("", "'; DROP TABLE books;--", 20, 0)
	if len(byBad) != 2 || byBad[0].ID != 10 {
		t.Fatalf("unknown sort should fall back to new [10,11], got %+v", ids(byBad))
	}
}

func ids(bs []CommunityBook) []int64 {
	out := make([]int64, len(bs))
	for i, b := range bs {
		out[i] = b.ID
	}
	return out
}
```

同时**更新**该文件里既有对 `ListPublic` 的调用（原 `repo.ListPublic("", 20, 0)` 之类）为新签名 `repo.ListPublic("", "new", 20, 0)`（`TestListPublicOrdersAndFiltersPrivate` 里那处）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/community/ -run TestListPublicSort -v`
Expected: FAIL（ListPublic 参数个数不符 / 未定义）

- [ ] **Step 3: 实现 repo** — 编辑 `internal/community/repo.go`

在 `likeUserID` 之后新增白名单函数：

```go
// orderClause maps a caller-supplied sort key to a FIXED, whitelisted ORDER BY
// clause. The sort value is NEVER interpolated into SQL: unknown or empty keys
// fall back to newest-first. Only "hot" (most-liked) diverges.
func orderClause(sort string) string {
	if sort == "hot" {
		return `ORDER BY b.like_count DESC, b.published_at DESC, b.id DESC`
	}
	return `ORDER BY b.published_at DESC, b.id DESC`
}
```

把 `ListPublic` 签名与查询改为（`sort` 参在 `viewerKey` 之后；ORDER BY 用白名单常量拼接，`sort` 值本身不进 SQL）：

```go
func (r *Repo) ListPublic(viewerKey, sort string, limit, offset int) ([]CommunityBook, error) {
	uid := likeUserID(viewerKey)
	rows, err := r.db.Query(
		`SELECT b.id, b.title, b.cover_url, b.summary, b.like_count, b.view_count, b.published_at,
		        u.nickname, u.avatar_url,
		        EXISTS(SELECT 1 FROM book_likes l WHERE l.book_id = b.id AND l.user_id = ?) AS liked
		   FROM books b JOIN users u ON u.id = b.user_id
		  WHERE b.is_public = 1
		  `+orderClause(sort)+`
		  LIMIT ? OFFSET ?`,
		uid, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list public books: %w", err)
	}
	defer rows.Close()

	out := make([]CommunityBook, 0)
	for rows.Next() {
		var c CommunityBook
		if err := rows.Scan(
			&c.ID, &c.Title, &c.CoverURL, &c.Summary, &c.LikeCount, &c.ViewCount, &c.PublishedAt,
			&c.Author.Nickname, &c.Author.AvatarURL, &c.Liked,
		); err != nil {
			return nil, fmt.Errorf("list public books: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list public books: %w", err)
	}
	return out, nil
}
```

- [ ] **Step 4: 更新 service** — 编辑 `internal/community/service.go` 的 `ListPublic`

```go
// ListPublic returns the public feed. sort is passed through to the repo, which
// whitelists it (unknown -> newest-first). limit/offset are clamped.
func (s *Service) ListPublic(viewerKey, sort string, limit, offset int) ([]CommunityBook, error) {
	limit, offset = clampPaging(limit, offset)
	return s.repo.ListPublic(viewerKey, sort, limit, offset)
}
```

- [ ] **Step 5: 更新 handler** — 编辑 `internal/community/handler.go` 的 `List`

在读取 limit/offset 附近加读取 `sort` 并传入：

```go
	sort := r.URL.Query().Get("sort")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	list, err := h.svc.ListPublic(vk, sort, limit, offset)
```

（`vk` 为该 handler 内既有的 viewerKey 变量；保持其余不变。）

- [ ] **Step 6: 跑测试确认通过 + 回归**

Run: `go test ./internal/community/ -v && go vet ./internal/community/`
Expected: PASS（含既有 read/write/handler 用例不回归）

- [ ] **Step 7: 提交**

```bash
git add internal/community/
git commit -m "feat: 社区 feed 加 sort 白名单参数(最新/最热)"
```

---

## Task 2: openapi `sort` 参数 + 契约 E2E

**Files:**
- Modify: `docs/openapi.yaml`（`GET /community/books` 加 `sort` query 参）
- Modify: `test/contract/contract_test.go`（加 `?sort=hot` 校验）

**Interfaces:**
- Consumes: Task 1 的 `sort` 行为。
- Produces: openapi 里 `/community/books` 的 `sort` 可选 query 参（enum `[new,hot]`）；契约测试覆盖 `?sort=hot`。

- [ ] **Step 1: 读现状** — 打开 `docs/openapi.yaml`，定位 `GET /community/books` 的 `parameters`（已有 `limit`/`offset`）。

- [ ] **Step 2: 加 `sort` 参数** — 在该端点 `parameters` 列表追加：

```yaml
        - name: sort
          in: query
          required: false
          schema:
            type: string
            enum: [new, hot]
          description: 排序方式；new=最新发布(缺省)，hot=最多点赞。未知值回落 new。
```

- [ ] **Step 3: 加契约测试** — 在 `test/contract/contract_test.go` 里社区 feed 用例附近，追加一次带 `?sort=hot` 的请求并 `ValidateResponse`（仿照既有匿名 `GET /community/books` 用例，仅 URL 改为 `/api/v1/community/books?sort=hot`；播种数据不触发 AI）。

- [ ] **Step 4: 跑契约测试**

Run: `go test ./test/contract/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add docs/openapi.yaml test/contract/contract_test.go
git commit -m "docs: openapi 补 community feed sort 参数 + 契约校验"
```

---

## Task 3: 前端 client `listCommunity` 改 options 入参

**Files:**
- Modify: `web/src/api/types.ts`（加 `CommunitySort`）
- Modify: `web/src/api/client.ts`（`listCommunity` 改 options）
- Modify: `web/src/pages/Community.tsx`（更新调用以保持编译，本页稍后被 Task 9 删除）

**Interfaces:**
- Produces:
  - `export type CommunitySort = 'new' | 'hot'`
  - `api.listCommunity(opts?: { sort?: CommunitySort; limit?: number; offset?: number }): Promise<CommunityBook[]>`（默认 `sort='new'`,`limit=20`,`offset=0`）。

- [ ] **Step 1: 加类型** — `web/src/api/types.ts` 追加

```ts
export type CommunitySort = 'new' | 'hot'
```

- [ ] **Step 2: 改 client** — `web/src/api/client.ts` 把 `listCommunity` 改为

```ts
  // 社区公开 feed（分页 + 排序）。匿名可调。
  listCommunity: (opts: { sort?: CommunitySort; limit?: number; offset?: number } = {}) => {
    const { sort = 'new', limit = 20, offset = 0 } = opts
    return request<CommunityBook[]>(
      'GET',
      `/api/v1/community/books?sort=${sort}&limit=${limit}&offset=${offset}`,
    )
  },
```

文件顶部 import 补 `CommunitySort`（与既有 `CommunityBook` 等一起）。

- [ ] **Step 3: 更新现有调用** — `web/src/pages/Community.tsx` 把两处调用改为 options 形式：
  - 首屏：`api.listCommunity({ limit: PAGE_SIZE, offset: 0 })`
  - loadMore：`api.listCommunity({ limit: PAGE_SIZE, offset })`

- [ ] **Step 4: 构建校验**

Run: `cd web && npm run build && npx vitest run`
Expected: TS 编译通过 + 既有测试全绿

- [ ] **Step 5: 提交**

```bash
git add web/src/api/types.ts web/src/api/client.ts web/src/pages/Community.tsx
git commit -m "feat(web): listCommunity 改 options 入参(支持 sort)"
```

---

## Task 4: 左栏用户区 SideNavUser

**Files:**
- Create: `web/src/components/shell/SideNavUser.tsx`
- Test: `web/src/components/shell/SideNavUser.test.tsx`

**Interfaces:**
- Consumes: `useAuth()`（`user`/`isAuthed`/`logout`）、`mediaUrl`、react-router `Link`。
- Produces: `SideNavUser`（无 props）——**登录态**渲染头像(或昵称首字)+昵称+`⭐ 积分 N`+`个人资料`(→`/profile`)+`退出`；**未登录**渲染 `<Link to="/login">登录 / 注册</Link>`。

- [ ] **Step 1: 写失败测试** — `web/src/components/shell/SideNavUser.test.tsx`

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { SideNavUser } from './SideNavUser'

const mockAuth = { user: null as unknown, isAuthed: false, logout: vi.fn() }
vi.mock('../../auth/useAuth', () => ({ useAuth: () => mockAuth }))

function renderUser() {
  return render(<MemoryRouter><SideNavUser /></MemoryRouter>)
}

describe('SideNavUser', () => {
  it('未登录显示登录/注册', () => {
    mockAuth.user = null
    mockAuth.isAuthed = false
    renderUser()
    const link = screen.getByRole('link', { name: /登录|注册/ })
    expect(link).toHaveAttribute('href', '/login')
  })

  it('登录态显示昵称与积分', () => {
    mockAuth.user = { id: 1, nickname: '小明', credits: 42, avatarUrl: '' }
    mockAuth.isAuthed = true
    renderUser()
    expect(screen.getByText('小明')).toBeInTheDocument()
    expect(screen.getByText(/42/)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /登录|注册/ })).not.toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/components/shell/SideNavUser.test.tsx`
Expected: FAIL（Cannot find module './SideNavUser'）

- [ ] **Step 3: 实现** — `web/src/components/shell/SideNavUser.tsx`

```tsx
import { Link } from 'react-router-dom'
import { useAuth } from '../../auth/useAuth'
import { mediaUrl } from '../../api/media'

// 左栏底部用户区：登录态显示头像/昵称/积分/资料/退出；未登录显示登录·注册入口。
export function SideNavUser() {
  const { user, isAuthed, logout } = useAuth()

  if (!isAuthed || !user) {
    return (
      <Link
        to="/login"
        className="block rounded-2xl bg-gradient-to-b from-sun to-peach px-4 py-3 text-center font-display font-bold text-white shadow-soft-sm transition-transform hover:-translate-y-0.5"
      >
        登录 / 注册
      </Link>
    )
  }

  const initial = user.nickname?.trim()?.[0] ?? '🙂'
  const avatar = mediaUrl(user.avatarUrl)

  return (
    <div className="flex flex-col gap-3 rounded-2xl bg-white/60 p-3">
      <div className="flex items-center gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-b from-sun to-peach font-display font-bold text-white">
          {avatar ? <img src={avatar} alt="头像" className="h-full w-full object-cover" /> : initial}
        </span>
        <div className="min-w-0">
          <p className="truncate font-display font-bold text-ink">{user.nickname}</p>
          <p className="text-xs font-semibold text-ink-soft" title="图像积分：出图/漫画化各扣 1 分，失败退还">
            ⭐ 积分 {user.credits}
          </p>
        </div>
      </div>
      <div className="flex gap-2">
        <Link
          to="/profile"
          className="flex-1 rounded-xl px-2 py-1.5 text-center font-display text-sm font-semibold text-ink-soft hover:bg-sky/10"
        >
          个人资料
        </Link>
        <button
          type="button"
          onClick={() => logout()}
          className="flex-1 rounded-xl px-2 py-1.5 text-center font-display text-sm font-semibold text-coral hover:bg-coral/10"
        >
          退出
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: 跑测试 + 构建**

Run: `cd web && npx vitest run src/components/shell/SideNavUser.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 5: 提交**

```bash
git add web/src/components/shell/SideNavUser.tsx web/src/components/shell/SideNavUser.test.tsx
git commit -m "feat(web): 左栏用户区 SideNavUser(登录态/未登录)"
```

---

## Task 5: 左栏 SideNav + 应用壳 AppShell

**Files:**
- Create: `web/src/components/shell/SideNav.tsx`
- Create: `web/src/components/shell/AppShell.tsx`
- Test: `web/src/components/shell/SideNav.test.tsx`

**Interfaces:**
- Consumes: `SideNavUser`（Task 4）、react-router `NavLink`/`Link`/`Outlet`。
- Produces:
  - `SideNav`（无 props）：Logo(→`/community`) + 两个 `NavLink` tab（`🌈 社区`→`/community`、`📚 我的漫画`→`/my`，active 高亮）+ 底部 `SideNavUser`。桌面竖排、窄屏顶部横排（Tailwind `md:` 控制）。
  - `AppShell`（无 props）：外层 flex 容器，左 `SideNav` + 右 `<main><Outlet/></main>`。

- [ ] **Step 1: 写失败测试** — `web/src/components/shell/SideNav.test.tsx`

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { SideNav } from './SideNav'

vi.mock('../../auth/useAuth', () => ({
  useAuth: () => ({ user: null, isAuthed: false, logout: vi.fn() }),
}))

describe('SideNav', () => {
  it('渲染社区与我的漫画两个 tab，指向正确路由', () => {
    render(<MemoryRouter><SideNav /></MemoryRouter>)
    expect(screen.getByRole('link', { name: /社区/ })).toHaveAttribute('href', '/community')
    expect(screen.getByRole('link', { name: /我的漫画/ })).toHaveAttribute('href', '/my')
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/components/shell/SideNav.test.tsx`
Expected: FAIL（Cannot find module './SideNav'）

- [ ] **Step 3: 实现 SideNav** — `web/src/components/shell/SideNav.tsx`

```tsx
import { NavLink, Link } from 'react-router-dom'
import type { ReactNode } from 'react'
import { SideNavUser } from './SideNavUser'

function Tab({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        [
          'flex items-center gap-2 rounded-2xl px-4 py-3 font-display font-bold transition-colors',
          isActive ? 'bg-white text-ink shadow-soft-sm' : 'text-ink-soft hover:bg-white/50',
        ].join(' ')
      }
    >
      {children}
    </NavLink>
  )
}

// 左栏：Logo + 两个 tab + 底部用户区。桌面竖排(md:)，窄屏收成顶部横排。
export function SideNav() {
  return (
    <nav className="flex shrink-0 flex-row items-center gap-3 border-b border-ink/5 bg-cream/80 px-4 py-3 backdrop-blur md:h-screen md:w-[230px] md:flex-col md:items-stretch md:border-b-0 md:border-r md:py-6">
      <Link to="/community" className="font-display text-xl font-extrabold text-ink md:mb-4 md:px-2">
        🎨 oh-my-commic
      </Link>
      <div className="flex flex-1 flex-row gap-2 md:flex-col md:flex-none">
        <Tab to="/community">🌈 社区</Tab>
        <Tab to="/my">📚 我的漫画</Tab>
      </div>
      <div className="md:mt-auto">
        <SideNavUser />
      </div>
    </nav>
  )
}
```

- [ ] **Step 4: 实现 AppShell** — `web/src/components/shell/AppShell.tsx`

```tsx
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
```

- [ ] **Step 5: 跑测试 + 构建**

Run: `cd web && npx vitest run src/components/shell/ && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 6: 提交**

```bash
git add web/src/components/shell/SideNav.tsx web/src/components/shell/AppShell.tsx web/src/components/shell/SideNav.test.tsx
git commit -m "feat(web): 应用壳 AppShell + 左栏 SideNav(tab+用户区,响应式)"
```

---

## Task 6: 社区展示件 HeroBanner + SortToggle + FeaturedRow

**Files:**
- Create: `web/src/components/community/HeroBanner.tsx`
- Create: `web/src/components/community/SortToggle.tsx`
- Create: `web/src/components/community/FeaturedRow.tsx`
- Test: `web/src/components/community/SortToggle.test.tsx`、`web/src/components/community/FeaturedRow.test.tsx`

**Interfaces:**
- Consumes: `CommunityBook`、`CommunitySort`、`CommunityCard`、`mediaUrl`、react-router `Link`。
- Produces:
  - `HeroBanner`（无 props）：欢迎横幅。
  - `SortToggle({ value, onChange }: { value: CommunitySort; onChange: (v: CommunitySort) => void })`：最新/最热分段控件。
  - `FeaturedRow({ books }: { books: CommunityBook[] })`：一排放大卡（每张 `<Link to={/community/books/${id}}>`，标题 + 作者 + ❤/👁）。

- [ ] **Step 1: 写失败测试** — `web/src/components/community/SortToggle.test.tsx`

```tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SortToggle } from './SortToggle'

describe('SortToggle', () => {
  it('点击切换触发 onChange', () => {
    const onChange = vi.fn()
    render(<SortToggle value="new" onChange={onChange} />)
    fireEvent.click(screen.getByRole('button', { name: /最热/ }))
    expect(onChange).toHaveBeenCalledWith('hot')
  })
})
```

`web/src/components/community/FeaturedRow.test.tsx`

```tsx
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { FeaturedRow } from './FeaturedRow'
import type { CommunityBook } from '../../api/types'

const book = (id: number, title: string): CommunityBook => ({
  id, title, coverUrl: '', summary: '梗概',
  author: { nickname: '小明', avatarUrl: '' },
  likeCount: 9, viewCount: 3, liked: false, publishedAt: 't',
})

describe('FeaturedRow', () => {
  it('渲染每本精选书的标题与阅读链接', () => {
    render(
      <MemoryRouter>
        <FeaturedRow books={[book(1, '甲'), book(2, '乙')]} />
      </MemoryRouter>,
    )
    expect(screen.getByText('甲')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /乙/ })).toHaveAttribute('href', '/community/books/2')
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/components/community/`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 实现 SortToggle** — `web/src/components/community/SortToggle.tsx`

```tsx
import type { CommunitySort } from '../../api/types'

const OPTIONS: { value: CommunitySort; label: string }[] = [
  { value: 'new', label: '最新' },
  { value: 'hot', label: '最热' },
]

// 最新/最热分段切换控件。
export function SortToggle({
  value,
  onChange,
}: {
  value: CommunitySort
  onChange: (v: CommunitySort) => void
}) {
  return (
    <div className="inline-flex rounded-full bg-white/70 p-1 shadow-soft-sm">
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
          className={[
            'rounded-full px-4 py-1.5 font-display text-sm font-bold transition-colors',
            value === o.value ? 'bg-gradient-to-b from-sun to-peach text-white' : 'text-ink-soft',
          ].join(' ')}
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: 实现 FeaturedRow** — `web/src/components/community/FeaturedRow.tsx`

```tsx
import { Link } from 'react-router-dom'
import type { CommunityBook } from '../../api/types'
import { mediaUrl } from '../../api/media'

// 精选一排：放大卡，突出封面 + 标题 + 作者 + 点赞/浏览。
export function FeaturedRow({ books }: { books: CommunityBook[] }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      {books.map((b) => {
        const cover = mediaUrl(b.coverUrl)
        return (
          <Link
            key={b.id}
            to={`/community/books/${b.id}`}
            className="group flex flex-col overflow-hidden rounded-3xl bg-white shadow-soft-sm transition-transform hover:-translate-y-1"
          >
            <div className="flex aspect-[16/9] items-center justify-center bg-gradient-to-br from-sky/20 to-peach/20 text-4xl">
              {cover ? (
                <img src={cover} alt={b.title} className="h-full w-full object-cover" />
              ) : (
                <span aria-hidden>📖</span>
              )}
            </div>
            <div className="flex flex-col gap-1 p-4">
              <h3 className="truncate font-display text-lg font-extrabold text-ink">{b.title}</h3>
              <p className="truncate text-sm text-ink-soft">by {b.author.nickname}</p>
              <p className="text-sm text-ink-soft">❤ {b.likeCount}　👁 {b.viewCount}</p>
            </div>
          </Link>
        )
      })}
    </div>
  )
}
```

- [ ] **Step 5: 实现 HeroBanner** — `web/src/components/community/HeroBanner.tsx`

```tsx
// 社区欢迎横幅：暖色渐变 + slogan，把内容区顶部填满。
export function HeroBanner() {
  return (
    <section className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-sun/30 via-peach/25 to-sky/20 px-8 py-10">
      <div className="relative z-10 max-w-2xl">
        <h1 className="font-display text-3xl font-extrabold text-ink sm:text-4xl">
          和小朋友一起，读一本别人画的魔法漫画 🌈
        </h1>
        <p className="mt-3 font-body text-ink-soft">
          这里是大家公开的绘本作品，翻一翻，说不定就遇见今晚的睡前故事。
        </p>
      </div>
      <span className="pointer-events-none absolute -right-2 top-2 text-6xl opacity-40" aria-hidden>☁️</span>
      <span className="pointer-events-none absolute bottom-2 right-16 text-3xl opacity-40" aria-hidden>⭐</span>
    </section>
  )
}
```

- [ ] **Step 6: 跑测试 + 构建**

Run: `cd web && npx vitest run src/components/community/ && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 7: 提交**

```bash
git add web/src/components/community/
git commit -m "feat(web): 社区展示件 HeroBanner/SortToggle/FeaturedRow"
```

---

## Task 7: 社区内容区 CommunityView（精选+排序+网格+去重+空态）

**Files:**
- Create: `web/src/pages/CommunityView.tsx`
- Test: `web/src/pages/CommunityView.test.tsx`

**Interfaces:**
- Consumes: `api.listCommunity`（Task 3）、`HeroBanner`/`SortToggle`/`FeaturedRow`（Task 6）、`CommunityCard`、`CommunitySort`、`CommunityBook`、`EmptyState`/`LoadingClouds`/`Button`、`errorMessage`。
- Produces: 默认导出 `CommunityView`（无 props）。行为：
  - 首屏并行拉：精选 `listCommunity({sort:'hot', limit:3})`；网格 `listCommunity({sort, limit:20, offset:0})`（`sort` 初始 `'new'`）。
  - **精选显示条件**：`featured.length === 3 && gridFirstPageLen > 3` 才渲染精选栏（`gridFirstPageLen` = 网格首屏去重前长度）。
  - **网格去重**：渲染时排除精选 id：`grid.filter(b => !featuredIds.has(b.id))`（仅当精选栏显示时）。
  - 排序切换：改 `sort` → 重拉网格（offset 归零、done 复位）；精选不变。
  - 加载更多、空态、错误：沿用现 Community 逻辑。

- [ ] **Step 1: 写失败测试** — `web/src/pages/CommunityView.test.tsx`

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../api/client', () => ({
  api: { listCommunity: vi.fn() },
}))
import { api } from '../api/client'
import CommunityView from './CommunityView'
import type { CommunityBook } from '../api/types'

const mk = (id: number, title: string, like = 0): CommunityBook => ({
  id, title, coverUrl: '', summary: 's',
  author: { nickname: '小明', avatarUrl: '' },
  likeCount: like, viewCount: 0, liked: false, publishedAt: 't',
})

describe('CommunityView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('精选(最热3)+网格去重：网格不重复渲染精选书', async () => {
    const featured = [mk(1, '热一', 9), mk(2, '热二', 8), mk(3, '热三', 7)]
    const grid = [mk(1, '热一', 9), mk(2, '热二', 8), mk(3, '热三', 7), mk(4, '普通四')]
    ;(api.listCommunity as unknown as ReturnType<typeof vi.fn>).mockImplementation(
      (opts: { sort?: string; limit?: number }) =>
        Promise.resolve(opts?.sort === 'hot' && opts?.limit === 3 ? featured : grid),
    )
    render(<MemoryRouter><CommunityView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('普通四')).toBeInTheDocument())
    // 精选区标题存在（说明精选栏渲染）
    expect(screen.getByText(/精选/)).toBeInTheDocument()
    // “热一” 只出现在精选，不在网格重复 → 页面中仅 1 处
    expect(screen.getAllByText('热一')).toHaveLength(1)
  })

  it('公开书≤3 时不渲染精选栏', async () => {
    const three = [mk(1, 'A'), mk(2, 'B'), mk(3, 'C')]
    ;(api.listCommunity as unknown as ReturnType<typeof vi.fn>).mockImplementation(
      (opts: { limit?: number }) => Promise.resolve(opts?.limit === 3 ? three : three),
    )
    render(<MemoryRouter><CommunityView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('A')).toBeInTheDocument())
    expect(screen.queryByText(/精选/)).not.toBeInTheDocument()
  })

  it('切换到最热重拉网格', async () => {
    ;(api.listCommunity as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([mk(1, '甲')])
    render(<MemoryRouter><CommunityView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('甲')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /最热/ }))
    await waitFor(() =>
      expect(api.listCommunity).toHaveBeenCalledWith(
        expect.objectContaining({ sort: 'hot', offset: 0 }),
      ),
    )
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/pages/CommunityView.test.tsx`
Expected: FAIL（Cannot find module './CommunityView'）

- [ ] **Step 3: 实现** — `web/src/pages/CommunityView.tsx`

```tsx
import { useEffect, useRef, useState } from 'react'
import { CommunityCard } from '../components/CommunityCard'
import { HeroBanner } from '../components/community/HeroBanner'
import { SortToggle } from '../components/community/SortToggle'
import { FeaturedRow } from '../components/community/FeaturedRow'
import { Button, EmptyState, LoadingClouds } from '../components/ui'
import { api } from '../api/client'
import type { CommunityBook, CommunitySort } from '../api/types'
import { errorMessage } from '../api/errors'

const PAGE_SIZE = 20
const FEATURED_N = 3

// 社区内容区：欢迎横幅 + 精选(最热3) + 排序切换 + 卡片网格 + 空态。
export default function CommunityView() {
  const [featured, setFeatured] = useState<CommunityBook[]>([])
  const [showFeatured, setShowFeatured] = useState(false)
  const [items, setItems] = useState<CommunityBook[]>([])
  const [sort, setSort] = useState<CommunitySort>('new')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [offset, setOffset] = useState(0)
  const [done, setDone] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const busyRef = useRef(false)

  // 首屏：并行拉精选(最热3)与网格(当前 sort)。
  useEffect(() => {
    let alive = true
    ;(async () => {
      setLoading(true)
      const featuredP = api
        .listCommunity({ sort: 'hot', limit: FEATURED_N })
        .catch(() => [] as CommunityBook[]) // 精选失败静默隐藏，不阻断网格
      const gridP = api.listCommunity({ sort, limit: PAGE_SIZE, offset: 0 })
      try {
        const [feat, grid] = await Promise.all([featuredP, gridP])
        if (!alive) return
        const gridList = grid ?? []
        const show = (feat ?? []).length === FEATURED_N && gridList.length > FEATURED_N
        setFeatured(feat ?? [])
        setShowFeatured(show)
        setItems(gridList)
        setOffset(gridList.length)
        setDone(gridList.length < PAGE_SIZE)
      } catch (err) {
        if (alive) setError(errorMessage(err))
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
    // 仅首屏拉一次；排序切换走 changeSort 单独重拉网格。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const changeSort = async (next: CommunitySort) => {
    if (next === sort) return
    setSort(next)
    setError('')
    busyRef.current = true
    try {
      const grid = await api.listCommunity({ sort: next, limit: PAGE_SIZE, offset: 0 })
      const list = grid ?? []
      setItems(list)
      setOffset(list.length)
      setDone(list.length < PAGE_SIZE)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      busyRef.current = false
    }
  }

  const loadMore = async () => {
    if (busyRef.current || done) return
    busyRef.current = true
    setLoadingMore(true)
    setError('')
    try {
      const grid = await api.listCommunity({ sort, limit: PAGE_SIZE, offset })
      const list = grid ?? []
      setItems((prev) => [...prev, ...list])
      setOffset((prev) => prev + list.length)
      if (list.length < PAGE_SIZE) setDone(true)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setLoadingMore(false)
      busyRef.current = false
    }
  }

  const featuredIds = new Set(featured.map((b) => b.id))
  const gridItems = showFeatured ? items.filter((b) => !featuredIds.has(b.id)) : items

  return (
    <div className="mx-auto max-w-6xl px-6 py-8">
      <HeroBanner />

      {loading ? (
        <div className="mt-8">
          <LoadingClouds label="正在打开社区…" />
        </div>
      ) : error && items.length === 0 ? (
        <div className="mt-8">
          <EmptyState emoji="🌧️" title="社区没打开" description={error} />
        </div>
      ) : items.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            emoji="🎨"
            title="还没有公开的漫画"
            description="还没有公开的漫画，去创作并发布第一本吧～"
          />
        </div>
      ) : (
        <>
          {showFeatured && (
            <section className="mt-8">
              <h2 className="mb-3 font-display text-xl font-extrabold text-ink">✨ 精选</h2>
              <FeaturedRow books={featured} />
            </section>
          )}

          <div className="mt-8 flex items-center justify-between">
            <h2 className="font-display text-xl font-extrabold text-ink">全部作品</h2>
            <SortToggle value={sort} onChange={changeSort} />
          </div>

          <div className="mt-4 grid grid-cols-2 gap-5 sm:grid-cols-3 lg:grid-cols-4">
            {gridItems.map((book) => (
              <CommunityCard key={book.id} book={book} />
            ))}
          </div>

          {error && <p className="mt-6 text-center text-sm font-semibold text-coral">{error}</p>}

          {!done && (
            <div className="mt-8 flex justify-center">
              <Button variant="ghost" onClick={loadMore} loading={loadingMore}>
                加载更多
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 4: 跑测试 + 构建**

Run: `cd web && npx vitest run src/pages/CommunityView.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 5: 提交**

```bash
git add web/src/pages/CommunityView.tsx web/src/pages/CommunityView.test.tsx
git commit -m "feat(web): 社区内容区 CommunityView(hero+精选+排序+网格去重+空态)"
```

---

## Task 8: 我的书架内容区 MyBooksView

**Files:**
- Create: `web/src/pages/MyBooksView.tsx`
- Test: `web/src/pages/MyBooksView.test.tsx`

**Interfaces:**
- Consumes: `api.get`/`api.setVisibility`/`api.del`/`api.post`、`BookCard`、`CreateCard`/`CreateBookModal`、UI 组件、`useSubmitOnce`。
- Produces: 默认导出 `MyBooksView`（无 props）——即现 `Bookshelf` 的**整段内容但去掉最外层 `<AppHeader/>` 与 `min-h-screen bg-cream` 外壳**（壳由 AppShell 提供）。保留：加载书籍、创建、删除确认、公开开关。

- [ ] **Step 1: 写失败测试** — `web/src/pages/MyBooksView.test.tsx`（沿用原 Bookshelf.test 的 mock，仅改导入与组件名）

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../api/client', () => ({
  api: {
    get: vi.fn().mockResolvedValue([
      { id: 1, userId: 1, title: '书A', coverUrl: '', style: 'ghibli', summary: '',
        isPublic: false, createdAt: 't', updatedAt: 't', likeCount: 0, viewCount: 0, publishedAt: '' },
    ]),
    setVisibility: vi.fn().mockResolvedValue({
      id: 1, userId: 1, title: '书A', coverUrl: '', style: 'ghibli', summary: '',
      isPublic: true, createdAt: 't', updatedAt: 't', likeCount: 0, viewCount: 0, publishedAt: 't2',
    }),
  },
}))
import { api } from '../api/client'
import MyBooksView from './MyBooksView'

describe('MyBooksView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('加载书籍并可切换公开', async () => {
    render(<MemoryRouter><MyBooksView /></MemoryRouter>)
    await waitFor(() => expect(screen.getByText('书A')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /公开|发布/ }))
    await waitFor(() => expect(api.setVisibility).toHaveBeenCalledWith(1, true))
  })
})
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd web && npx vitest run src/pages/MyBooksView.test.tsx`
Expected: FAIL（Cannot find module './MyBooksView'）

- [ ] **Step 3: 实现** — 新建 `web/src/pages/MyBooksView.tsx`：把现 `web/src/pages/Bookshelf.tsx` 的内容整体拷入并改造：
  - 默认导出改名 `MyBooksView`。
  - **去掉** `import { AppHeader }`，**去掉** JSX 里最外层 `<div className="min-h-screen bg-cream"> <AppHeader/> ... </div>`，直接返回原来的 `<main>...</main>`（保留 `<main className="mx-auto max-w-6xl px-6 py-8">` 及其内所有内容、以及页面级的 `<CreateBookModal/>`/删除 `<Modal/>`——把它们包在一个 `<>...</>` fragment 里）。
  - `CreateCard`/`CreateBookModal` 两个内部组件一并保留在本文件。
  - 其余逻辑（`useNavigate`、加载、`toggleVisibility`、删除确认）原样保留。
  - 不改任何 API 调用路径。

- [ ] **Step 4: 跑测试 + 构建**

Run: `cd web && npx vitest run src/pages/MyBooksView.test.tsx && npm run build`
Expected: PASS + 构建通过

- [ ] **Step 5: 提交**

```bash
git add web/src/pages/MyBooksView.tsx web/src/pages/MyBooksView.test.tsx
git commit -m "feat(web): 我的书架内容区 MyBooksView(从 Bookshelf 抽出,去顶栏)"
```

---

## Task 9: 路由接壳 + 删除旧页

**Files:**
- Modify: `web/src/App.tsx`
- Delete: `web/src/pages/Home.tsx`、`web/src/pages/Home.test.tsx`、`web/src/pages/Community.tsx`、`web/src/pages/Bookshelf.tsx`、`web/src/pages/Bookshelf.test.tsx`

**Interfaces:**
- Consumes: `AppShell`（Task 5）、`CommunityView`（Task 7）、`MyBooksView`（Task 8）、`CommunityReader`、`RequireAuth`。
- Produces: 新路由树——`AppShell` 为无路径 layout route，其下 `/`（重定向 `/community`）、`/community`→`CommunityView`、`/my`→`Protected MyBooksView`；`/community/books/:id`→`CommunityReader`（全屏，壳外）；其余整页不变。

- [ ] **Step 1: 改 App.tsx** — 替换为：

```tsx
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
```

- [ ] **Step 2: 删除旧页与测试**

```bash
git rm web/src/pages/Home.tsx web/src/pages/Home.test.tsx web/src/pages/Community.tsx web/src/pages/Bookshelf.tsx web/src/pages/Bookshelf.test.tsx
```

- [ ] **Step 3: 全量回归 + 构建**（确认无悬空 import：其它文件若 `import Bookshelf`/`Home`/`Community` 需清理；正常只有 App.tsx 引用它们）

Run: `cd web && npm run build && npx vitest run`
Expected: TS 编译通过（无 "Cannot find module './pages/Home'" 之类）+ 全部测试绿

- [ ] **Step 4: 提交**

```bash
git add web/src/App.tsx
git commit -m "feat(web): 路由接入 AppShell(/, /community, /my 进壳; 阅读器全屏), 删除旧 Home/Community/Bookshelf"
```

---

## Task 10: E2E 更新（community.spec + smoke.spec）

**Files:**
- Modify: `web/e2e/community.spec.ts`
- Modify: `web/e2e/smoke.spec.ts`

**Interfaces:**
- Consumes: 运行中的服务（e2e 既有启动方式）。
- Produces: E2E 匹配新壳布局与路由。

- [ ] **Step 1: 读现状** — 打开 `web/e2e/community.spec.ts`、`web/e2e/smoke.spec.ts`，找出对 `/`、`/community`、`/my`、Home 两卡文案的断言。

- [ ] **Step 2: 更新 community.spec.ts**：
  - 匿名用例：`page.goto('/community')` → 断言页面出现社区内容（如 hero slogan「读一本别人画的魔法漫画」或「全部作品」标题），且左栏「社区」tab 存在（`getByRole('link', { name: /社区/ })`）。未被重定向到 `/login`。
  - owner 发布用例：注册登录落 `/my`（现在 `/my` 在壳内，仍显示「我的书架」标题）→ 建书 → 点「设为公开」→ `page.goto('/community')` → 断言书标题出现在网格。
  - 若之前断言的社区页文案（如旧「社区 🌈」heading）变化，改为新文案（`全部作品` 或 hero slogan）。

- [ ] **Step 3: 更新 smoke.spec.ts**：
  - `/` 现在**重定向到 `/community`**（公开）。断言 `page.goto('/')` 后 URL 落 `/community` 或社区内容可见；**不再**有旧「两卡 Home」文案。
  - 未登录访问 `/my` → 仍跳 `/login`（受保护）。
  - 登录/注册成功仍落 `/my`（书架），断言「我的书架」可见。
  - 删掉任何针对已删除 Home 页文案（如两入口卡）的断言。

- [ ] **Step 4: 实跑（起 fresh 服务）**

Run（务必先确认 8080 无残留旧二进制）：
```
cd web && npm run build
# 另起：构建并运行 fresh 二进制(假 key 即可，社区流程不碰 AI)
# DB_PATH=/tmp/e2e.db DATA_DIR=/tmp/e2e_data PORT=8080 WEB_DIST=web/dist \
#   DASHSCOPE_API_KEY=sk-test ARK_API_KEY=ark-test INVITE_CODE=welcome123 \
#   ADMIN_USERNAME=admin ADMIN_PASSWORD=Passw0rd1 go build -o /tmp/e2e_omc ./cmd/server && /tmp/e2e_omc &
cd web && npx playwright test smoke.spec.ts community.spec.ts register.spec.ts
```
Expected: 全绿（跑完停掉后台服务）。若环境无法起服务/装浏览器，至少 `npx playwright test community.spec.ts smoke.spec.ts --list` 确认解析，并在报告注明未完整执行。

- [ ] **Step 5: 提交**

```bash
git add web/e2e/community.spec.ts web/e2e/smoke.spec.ts
git commit -m "test(web): E2E 适配应用壳(/, /community, /my)与 Home 删除"
```

---

## Task 11: 文档同步

**Files:**
- Modify: `CLAUDE.md`、`docs/frontend-api.md`、`docs/ARCHITECTURE-AND-PROMPTS.md`

**Interfaces:**
- Produces: 三处文档与新壳/路由/排序一致。

- [ ] **Step 1: CLAUDE.md**：更新前端路由描述——`/` 重定向到 `/community`；`AppShell` 应用壳（左栏 Logo+社区/我的漫画 tab+用户区）包住 `/community` 与 `/my`；公开阅读器 `/community/books/:id` 全屏独立；`Home` 两卡页已移除。前端关键组件补 `AppShell`/`SideNav`。

- [ ] **Step 2: docs/frontend-api.md**：`GET /community/books` 补 `sort`（`new`/`hot`，缺省 new）查询参，注明以 openapi.yaml 为准。

- [ ] **Step 3: docs/ARCHITECTURE-AND-PROMPTS.md**：「模块地图」前端部分——加 `components/shell/AppShell`、`SideNav`、`SideNavUser`；`components/community/HeroBanner`、`SortToggle`、`FeaturedRow`；`pages/CommunityView`、`pages/MyBooksView`；移除 `pages/Home`，把 `pages/Bookshelf`/`pages/Community` 标注为已并入 View。

- [ ] **Step 4: 提交**

```bash
git add CLAUDE.md docs/frontend-api.md docs/ARCHITECTURE-AND-PROMPTS.md
git commit -m "docs: 应用壳/路由/feed sort 写入 CLAUDE.md/frontend-api/架构文档"
```

---

## 收尾（执行完所有任务后）

- [ ] 后端：`CGO_ENABLED=1 go test -race ./... && go vet ./...`
- [ ] 前端：`cd web && npm run test && npm run build`
- [ ] `feat/community-shell` `--no-ff` 合并回 `main`，删分支。
