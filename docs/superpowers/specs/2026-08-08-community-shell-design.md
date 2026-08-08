# 社区应用壳 + 社区内容区重设计

> 日期 2026-08-08。目标：把「两张大卡的主页」重设计为**统一应用壳**——左侧常驻导航栏（Logo + 社区/我的漫画两个 tab + 用户区），右侧主内容区随 tab 切换；社区内容区做得优雅充实（欢迎横幅 + 精选 + 排序 + 卡片网格 + 空态）。
> 本 spec 经 brainstorming 对齐，作为 writing-plans 的输入。UI 以本文的结构图为准（用户不看前端代码，用页面结构图对齐）。

## 1. 目标与非目标

**目标**
- 统一应用壳 `AppShell`：左侧导航栏常驻，右侧主内容区随 tab 切换。
- 左栏含：Logo（→社区）、`社区 / 我的漫画` 两个 tab、底部用户区（头像/昵称/积分/资料/退出；未登录=登录/注册）。
- `/`、`/community`、`/my` 收进壳；公开阅读器 `/community/books/:id` 保持全屏沉浸（不进壳）。
- 社区内容区优雅充实：欢迎横幅(hero) + 精选(最热 3 本) + 排序切换(最新/最热) + 卡片网格 + 空态插画。
- 后端 feed 加 `sort` 白名单参数（最新/最热）。
- 交付同步：openapi 契约、文档、CI、测试各列独立任务。

**非目标（YAGNI）**
- 分类/标签/搜索、关注、评论、收藏。
- 精选轮播/自动播放（精选就是静态一排 3 张大卡）。
- 左栏可折叠/记忆宽度（桌面固定宽，窄屏收成顶部 tab 行）。
- 新增后端端点（精选复用 feed 的 `?sort=hot&limit=3`）。

## 2. 决策基线（已与用户确认）
- 壳范围：**统一应用壳**（不是只改 `/` 落地页）。
- 左栏内容：**Logo + 品牌** 与 **用户区（头像/昵称/积分）**（不放快捷「新建漫画」按钮——建书仍在「我的漫画」书架内）。
- 内容充实：**hero + 精选 + 排序切换 + 空态插画** 四项全要。
- UI 对齐：用结构图，不做浏览器 mockup。

## 3. 布局结构

### 3.1 桌面（≥ md）
```
┌────────────────┬─────────────────────────────────────────────┐
│ SideNav ~230px  │ 主内容区 (随 tab)                            │
│ 🎨 oh-my-commic  │  社区 tab → CommunityView(§4)                │
│ [🌈 社区]◀active │  我的漫画 tab → MyBooksView(书架)            │
│ [📚 我的漫画]    │                                             │
│ ───(spacer)───  │                                             │
│ 👤 用户区(底部)  │                                             │
│  头像 昵称       │                                             │
│  ⭐ 积分 N       │                                             │
│  资料 · 退出     │  (未登录: 登录/注册)                        │
└────────────────┴─────────────────────────────────────────────┘
```

### 3.2 窄屏（< md）
左栏收成**顶部一行**：`🎨logo  [社区][我的漫画]  [头像]`，其下为主内容区竖排。用 Tailwind 响应式断点（`md:`）实现，不做 JS 折叠。

### 3.3 组件拆分（小文件、单一职责）
- `components/shell/AppShell.tsx`：布局骨架（左栏 + 右侧 `<Outlet/>`）。
- `components/shell/SideNav.tsx`：左栏（Logo + NavTabs + 用户区）。桌面竖排 / 窄屏横排由 CSS 控制。
- `components/shell/SideNavUser.tsx`：用户区（登录态头像/昵称/积分/资料/退出；未登录=登录/注册）。从现有 `AppHeader` 的用户菜单逻辑抽取复用。
- `pages/CommunityView.tsx`：社区内容区（§4）。由现 `pages/Community.tsx` 的 body 演化而来。
- `pages/MyBooksView.tsx`：我的书架内容。由现 `pages/Bookshelf.tsx` 的 body 演化而来（去掉自带 AppHeader）。

## 4. 社区内容区（CommunityView）

从上到下四块：
```
✦ HERO 欢迎横幅：一句 slogan「和小朋友一起，读一本别人画的魔法漫画 🌈」+ 柔和暖色渐变 + 云朵/星星点缀
✨ 精选（最热 3 本，按点赞降序）：一排 3 张放大卡（FeaturedCard）
全部作品   [最新 · 最热] 排序切换（右上分段控件）
  CommunityCard 网格 + [加载更多]
空态：居中插画 🎨 +「还没有公开的漫画，去创作/发布第一本吧～」
```

### 4.1 数据装配与去重
- 首屏并行拉两次：精选 `listCommunity({sort:'hot', limit:3})`；网格 `listCommunity({sort:selected, limit:20, offset:0})`。
- **去重**：网格渲染时排除精选 3 本的 id（`grid.filter(b => !featuredIds.has(b.id))`）。
- **精选显示条件**：仅当**精选返回满 3 本** 且 **网格首页（去重前）返回 > 3 本**时才显示精选栏（说明去掉精选后网格仍有内容）；否则隐藏精选、只显示网格。这样公开书 ≤3 本时不会出现「精选=全部、网格空」的尴尬。
- 排序切换：切到「最热」重拉网格（`sort:'hot'`，offset 归零、done 复位）；精选栏不随排序变（恒为最热 3）。

### 4.2 组件
- `components/community/HeroBanner.tsx`：纯展示横幅。
- `components/community/FeaturedRow.tsx`：吃 `books: CommunityBook[]`，渲染放大卡（可内联小 `FeaturedCard` 或复用 CommunityCard 的放大变体）。
- `components/community/SortToggle.tsx`：`value: 'new'|'hot'` + `onChange`，分段控件。
- 复用现有 `CommunityCard`（网格卡）。
- 空态复用现有 `EmptyState`。

## 5. 后端：feed 排序参数

`GET /api/v1/community/books` 增可选查询参 `sort`：
- `sort=new`（缺省）→ `ORDER BY published_at DESC, id DESC`（现有行为，向后兼容）。
- `sort=hot` → `ORDER BY like_count DESC, published_at DESC, id DESC`。
- **安全**：`sort` 走**白名单**映射到固定 ORDER BY 子句；非法/缺省一律回落 `new`；**绝不把 `sort` 值拼进 SQL**。

**改动点**：
- `internal/community/repo.go` `ListPublic(viewerKey, sort, limit, offset)` 加 `sort string` 入参（内部 `orderClause(sort)` 白名单）。
- `internal/community/service.go` `ListPublic` 透传 `sort`（未知值不报错、回落 new）。
- `internal/community/handler.go` `List` 读 `r.URL.Query().Get("sort")` 传入。
- `docs/openapi.yaml`：`GET /community/books` 增 `sort` query 参（enum `[new,hot]`，可选）。
- `test/contract`：补 `?sort=hot` 一次 `ValidateResponse`。

## 6. 路由（`web/src/App.tsx`）

| 路径 | 渲染 | tab |
|---|---|---|
| `/` | 重定向到 `/community`（`<Navigate to="/community" replace/>`） | 社区 |
| `/community` | `AppShell` + `CommunityView` | 社区高亮 |
| `/my` | `<Protected>AppShell + MyBooksView</Protected>` | 我的漫画高亮 |
| `/community/books/:id` | `CommunityReader`（全屏，**不进壳**） | — |
| `/login`、`/profile`、`/books/*`、`/chapters/*`、`/read/*` | 各自整页（沿用 AppHeader） | — |

- 用嵌套路由：`AppShell` 作为 layout route（含 `<Outlet/>`），`/community` 与 `/my` 作为其子。`/my` 子路由包 `RequireAuth`。
- 左栏 tab 用 `NavLink`（`to="/community"` / `to="/my"`），`aria-current`/高亮由 `NavLink` 的 active 态驱动。
- **未登录点「我的漫画」**：`/my` 受保护 → `RequireAuth` 引导登录（现有行为）。左栏 tab 本身不禁用。
- 现 `pages/Home.tsx`（两卡）被壳取代 → **删除 Home 及其路由/测试**。
- 现 `pages/Community.tsx` / `pages/Bookshelf.tsx` 演化为 `CommunityView` / `MyBooksView`（去自带 AppHeader）。壳提供 chrome。

## 7. 错误处理
- CommunityView 首屏：精选与网格各自独立 try/catch；网格错误走 `EmptyState`+`errorMessage`，精选错误则**静默隐藏精选栏**（不阻断网格）。
- 加载更多错误：内联红字、保留已加载列表（沿用现有行为）。
- 后端 `sort` 非法 → 回落 `new`（不报错，容错）。
- 未登录访问 `/my` → RequireAuth 引导登录，不泄露。
- 沿用：公开只读隔离（is_public 过滤 + 404）、作者仅 nickname/avatar。

## 8. 测试（全 mock，不碰 AI/真 key）
- **后端单元**：`ListPublic` `sort=hot` 按 like_count 降序、`sort=new`/未知回落 published_at 降序、`orderClause` 白名单不受注入影响。
- **契约 E2E**：`GET /community/books?sort=hot` `ValidateResponse`。
- **前端 Vitest**：
  - `AppShell`/`SideNav`：渲染两 tab + 用户区；未登录显示登录/注册、登录态显示昵称/积分。
  - `SortToggle`：点击触发 onChange。
  - `CommunityView`：mock api，精选(最热3)+网格渲染、去重（网格不含精选 id）、精选<3 时不渲染精选栏、切排序重拉网格。
  - `FeaturedRow`/`HeroBanner`：渲染要素。
- **Playwright E2E**：更新既有 `community.spec.ts`——匿名进 `/community`（壳 + 社区内容区，「社区」tab active）；owner 发布后书出现在网格。更新 `smoke.spec.ts` 若因路由/Home 删除受影响。

## 9. 交付与维护（plan 必须含独立任务）
1. 契约：`sort` 参数进 openapi + 契约 E2E。
2. 文档：`CLAUDE.md`（前端路由/壳结构）、`docs/frontend-api.md`（feed `sort` 参）、`docs/ARCHITECTURE-AND-PROMPTS.md`（前端组件地图：AppShell/SideNav/CommunityView/MyBooksView/Hero/Featured/SortToggle；Home 删除）。
3. 测试：如上，列任务。
4. CI：新测试自动被 `ci.yml`/`e2e.yml` 覆盖。

## 10. 安全/隐私
- `sort` 白名单，绝不拼接用户输入进 SQL。
- 公开只读隔离不变（is_public + 404、作者仅 nickname/avatar、不含 conversation/panelCount）。
- 壳不改变鉴权：`/my` 仍受保护，公开社区浏览仍匿名可达。
