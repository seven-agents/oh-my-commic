# 社区功能设计（公开浏览 / 发布开关 / 点赞 / 浏览量）

> 日期 2026-08-08。目标：让**未登录访客**也能浏览并完整阅读用户「公开」的漫画，登录用户可管理自己的漫画并发布到社区。主页提供两个入口：**社区** 与 **我的漫画**。
> 本 spec 经 brainstorming 对齐，作为 writing-plans 的输入。

## 1. 目标与非目标

**目标**
- 未登录访客可浏览**社区 feed**（公开书网格）并**完整阅读**任一公开书（翻页阅读器，含所有章节页 + 图片）。
- owner 用**整本书级别的「公开/私密」开关**把书发布到社区 / 从社区下架（默认私密）。
- 社区卡片展示：封面、标题、**故事概述(summary)**、**作者昵称+头像**、**点赞数**、**浏览量**。
- **点赞**（需登录，幂等切换）与**浏览量=独立访客数**（防刷）。
- 主页两个入口：社区（公开）/ 我的漫画（需登录）。
- 交付必须**同步**：OpenAPI 契约、文档、测试、CI（各列独立任务）。

**非目标（YAGNI，本期不做）**
- 管理员治理（下架他人书 / 审核队列 / 举报）——仅 owner 自控。
- 评论、收藏夹、关注、分享外链。
- 按章节公开（只做整本公开）。
- 浏览量精确防刷（匿名去重用前端随机 id，可被清缓存绕过，本期够用）。

## 2. 决策基线（已与用户确认）
- 发布粒度：**整本书手动开关**，owner 控制，默认私密，**无管理员审核**。
- 内容治理：**本期不做**，仅 owner 自己改回私密。
- 卡片展示：作者昵称+头像、summary、点赞数、浏览量（**四项全要**）。
- 浏览量语义：**独立访客数**（非刷新次数），构造上防刷。
- 路由：`/` 变为**公开 Home**（两入口），原书架从 `/` 挪到 `/my`。

## 3. 架构

**后端**
- `book` 包新增一个 owner 写操作 `SetVisibility`（归属校验，写 `is_public` + `published_at`），保持「归属域写操作留在 book 包」。
- 新增 `internal/community` 包：`repo`（跨表**只读** SQL join，只查 `is_public=1`）+ `service` + `handler`。点赞 / 浏览计数在此包。
- 图片沿用已有的**公开** `/media/*` 静态服务（不在 `RequireUser` 后），匿名可直接加载，无需改动。
- BookReader 拆出纯展示组件，owner 私有阅读与社区公开阅读共用同一 UI，只换数据源（DRY）。

**隔离与隐私铁律（沿用）**
- 公开详情接口**只**返回 `is_public=1` 的书；非公开 / 不存在一律 **404**（不泄露私密书存在性）。
- 作者信息只暴露 `nickname` + `avatarUrl`，**绝不**返回 username / email / 其它账号字段。
- 点赞 / 浏览接口同样先校验目标书 `is_public`，否则 404。

## 4. 数据模型

### 4.1 `books` 新增列（幂等 `ALTER ADD COLUMN`；`is_public` 已存在）
| 列 | 类型 | 说明 |
|---|---|---|
| `like_count` | INTEGER NOT NULL DEFAULT 0 | 反范式点赞数，feed 直接读 |
| `view_count` | INTEGER NOT NULL DEFAULT 0 | 反范式独立访客数 |
| `published_at` | TEXT NOT NULL DEFAULT '' | 每次「公开」时写 `datetime('now')`；feed 按其倒序 |

- 索引：`CREATE INDEX IF NOT EXISTS idx_books_public_published ON books(is_public, published_at)`（feed 查询）。
- `models.Book` 相应增 `LikeCount int` / `ViewCount int` / `PublishedAt string`；`bookColumns` 常量与 `scanBook` 单点更新，覆盖所有现有 SELECT。列追加在末尾，扫描顺序同步。

### 4.2 新表（进 `schemaStatements`，`CREATE TABLE IF NOT EXISTS`）
```sql
CREATE TABLE IF NOT EXISTS book_likes (
  book_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (book_id, user_id),
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)
```
```sql
CREATE TABLE IF NOT EXISTS book_views (
  book_id INTEGER NOT NULL,
  viewer_key TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (book_id, viewer_key),
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
)
```
- `book_likes` 复合主键天然幂等：一个用户对一本书只算一次。
- `book_views.viewer_key` 去重键：登录用户 `u:{userId}`，匿名 `anon:{clientId}`（前端 localStorage 随机 id）。
- 计数一致性：like/unlike 与 view 均先写关系表，**仅当关系表实际发生插入/删除（RowsAffected==1）**才对 `books.like_count`/`view_count` 做 `+1`/`-1`，避免重复计数。同一事务内完成。

### 4.3 community 响应 DTO（community 包内，非 models.Book）
```go
type Author struct {
    Nickname  string `json:"nickname"`
    AvatarURL string `json:"avatarUrl"`
}
// CommunityBook 是 feed 列表项：book 子集 + 作者 + 当前用户是否已赞。
type CommunityBook struct {
    ID          int64  `json:"id"`
    Title       string `json:"title"`
    CoverURL    string `json:"coverUrl"`
    Summary     string `json:"summary"`
    Author      Author `json:"author"`
    LikeCount   int    `json:"likeCount"`
    ViewCount   int    `json:"viewCount"`
    Liked       bool   `json:"liked"`
    PublishedAt string `json:"publishedAt"`
}
```
- 阅读详情复用现有 `models.Book` + `[]models.Chapter` + 每章 `[]models.Panel`（阅读器已消费的结构），外加 `Author`、`likeCount`、`viewCount`、`liked`。定义 `CommunityBookDetail`。

## 5. API 端点（全部 `/api/v1`，同步进 `docs/openapi.yaml`）

### 5.1 公开（无需登录；handler 可选读 session 以填 `liked`）
| 方法 路径 | 说明 | 状态码 |
|---|---|---|
| `GET /api/v1/community/books?limit=&offset=` | 公开书列表，`published_at` 倒序。`limit` 缺省 20、上限 50；`offset≥0` | 200 |
| `GET /api/v1/community/books/{id}` | 公开阅读详情（book+chapters+panels+author+计数+liked）。非公开/不存在 → 404。**不改计数** | 200 / 404 |
| `POST /api/v1/community/books/{id}/view` | 记一次独立浏览。body `{clientId?: string}`（匿名去重用）。非公开/不存在 → 404 | 200 / 404 |

- 公开端点挂在 `/api/v1` 下、但**在 `RequireUser` 组之外**新建一个「可选鉴权」组：一个 `OptionalUser` 中间件，有有效 session 就把 userID 放进 context，无 session 也放行（不返回 401）。handler 用它区分 `liked` 与匿名 `viewer_key`。

### 5.2 需登录
| 方法 路径 | 说明 | 状态码 |
|---|---|---|
| `PUT /api/v1/books/{id}/visibility` | body `{isPublic: bool}`。owner 发布/下架，写 `is_public`+`published_at`（发布时置 now；下架保留旧值）。归属校验，非本人/不存在 404 | 200 / 400 / 404 |
| `POST /api/v1/community/books/{id}/like` | 点赞（幂等）。非公开/不存在 404。返回 `{likeCount, liked:true}` | 200 / 401 / 404 |
| `DELETE /api/v1/community/books/{id}/like` | 取消赞（幂等）。返回 `{likeCount, liked:false}` | 200 / 401 / 404 |

- `visibility` 端点归 `book` 包（owner 写、归属域）；like 归 `community` 包（社区语境），挂在 `RequireUser` 组。
- 前端对 like 的 401**单独处理**（引导登录），**不**触发全局登出。

## 6. 前端

### 6.1 路由（`web/src/App.tsx`）
| 路径 | 页面 | 鉴权 |
|---|---|---|
| `/` | `Home`（两入口：社区 / 我的漫画） | 公开 |
| `/community` | `Community`（公开书网格） | 公开 |
| `/community/books/:id` | 公开阅读器（复用 `BookReaderView` + 公开 API） | 公开 |
| `/my` | `Bookshelf`（原 `/`，每卡加公开开关） | 受保护 |
| `/login`、`/profile`、`/books/*`、`/chapters/*`、`/read/*` | 不变 | 不变 |

- 登录成功后默认导航到 `/my`（原逻辑落到 `/`，改为 `/my`）。
- `RequireAuth` 不变；社区三路由在 `Protected` 之外。

### 6.2 组件
- `pages/Home`：hero + 两个大入口卡；AppHeader 登录态显示头像、未登录显示「登录/注册」。
- `pages/Community`：`CommunityCard` 网格（封面、标题、summary、作者昵称+头像、❤likeCount、👁viewCount），点击进 `/community/books/:id`。分页「加载更多」（limit/offset）。
- `components/BookReaderView`：从现 `BookReader` 抽出的纯展示组件（输入 book+chapters+panels+author+计数）。`/books/:id/read`（owner 私有 API）与 `/community/books/:id`（公开 API）两个容器共用它。
- 公开阅读器容器：挂载时 `POST view`（带 localStorage clientId）；提供点赞按钮（未登录点 → 跳 `/login`）。
- `pages/Bookshelf`（移到 `/my`）：每张书卡加「公开/私密」`Toggle`（调 `PUT visibility`）+ 公开徽标 + like/view 计数展示。
- `api/client.ts`：`listCommunity(limit,offset)` / `getCommunityBook(id)` / `recordView(id, clientId)` / `likeBook(id)` / `unlikeBook(id)` / `setVisibility(id, isPublic)`。
- `api/types.ts`：`CommunityBook` / `CommunityBookDetail` / `Author` / `LikeResult`。
- clientId：`web/src/lib` 里一个 `getClientId()`，localStorage 缺失则生成随机串（`crypto.randomUUID()`）。

## 7. 错误处理
- 非公开 / 不存在书（feed 详情、view、like）一律 **404**，不泄露存在性。
- `visibility` 非 owner / 不存在 → 404；`isPublic` 缺失或非布尔 → 400。
- like/unlike 未登录 → 401（前端引导登录，不全局登出）。
- `limit`/`offset` 非法 → 夹到合法区间（不报错，容错）。
- 沿用：AI 上游 429/504/502；绝不泄露账号敏感字段 / 上游细节。

## 8. 交付与维护要求（**plan 必须含独立任务**）
1. **契约(API)**：新端点全部写入 `docs/openapi.yaml`（OpenAPI 3.1）；`test/contract` 契约 E2E（kin-openapi 校验真实响应）覆盖每个新端点。
2. **文档**：`CLAUDE.md`（社区模块 + 公开只读约定 + 路由变化）、`docs/frontend-api.md`（概览）、`docs/ARCHITECTURE-AND-PROMPTS.md`（模块地图加 community 包 / 前端页）同步更新。
3. **测试**：community repo / service 单元 + handler 集成 + 前端 Vitest + Playwright E2E，均列任务。
4. **CI**：新增测试自动被既有 `ci.yml`（`go test -race`、`npm run test`）与 `e2e.yml` 覆盖；E2E 需能在无 AI 的前提下跑（播种一本公开书，或用户发布一本仅含封面/文本的书）。

## 9. 测试要点（全 mock，不碰 AI / 真 key）
- **单元（community repo）**：只列 `is_public=1`；按 `published_at` 倒序；分页 limit/offset 边界；like 幂等（重复赞只 +1，重复取消不为负）；view 去重（同 viewer_key 只 +1）；`liked` 随当前用户变化。
- **单元（book）**：`SetVisibility` 归属校验（非 owner 返回 ErrNotFound）；发布置 `published_at`，下架不清空。
- **集成**：匿名（无 cookie）取 feed / 详情 200；私密书详情 404；view 去重跨请求只 +1；like 未登录 401、非公开 404、幂等；`OptionalUser` 有/无 session 行为。
- **契约 E2E**：feed / 详情 / view / visibility / like / unlike 逐个 `ValidateResponse`。
- **前端 Vitest**：Home 两入口渲染 + 跳转；CommunityCard 渲染作者/summary/计数；点赞按钮未登录跳登录。
- **Playwright E2E（AI-free）**：匿名访问 `/community` 看到播种公开书 → 打开阅读器翻页；登录用户在 `/my` 把一本书设为公开 → 出现在 `/community`。

## 10. 安全说明
- 公开只读端点严格 `is_public` 过滤 + 404 语义，杜绝私密书 / 作者账号信息泄露。
- 写操作（visibility / like）仍在 `RequireUser` 后，归属域校验不变。
- 浏览量为独立访客近似值，匿名去重可被绕过但不涉及安全，仅展示用途；不据此做配额或授权决策。
- 未引入用户原始上传数据的新存储，隐私约定不受影响。
