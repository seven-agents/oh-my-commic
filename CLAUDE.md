# CLAUDE.md — oh-my-commic 开发指南

面向 AI 亲子漫画书应用。本文件是**后续迭代的上手指南**（每次会话会加载）。详细的提示词原文见 [`docs/ARCHITECTURE-AND-PROMPTS.md`](docs/ARCHITECTURE-AND-PROMPTS.md)。

## 是什么
面向小朋友&亲子的 AI 漫画书：建书 → 建角色/场景（上传图，AI 漫画化成锁定形象）→ 讲故事对话拆镜 → 逐格 AI 解析 + 出图（吉卜力风）→ 拼成书翻页阅读。多用户、严格隔离。

## 技术栈
- **后端**：Go 单体，chi 路由，分层 `handler → service → repository`，SQLite（`modernc.org/sqlite` 纯 Go，CGO_ENABLED=0）+ 本地图片目录。
- **前端**：React + Vite + TS + Tailwind（SPA，生产由 Go 服务托管 `web/dist`）。
- **文本 LLM**：通义千问 `qwen-plus`（DashScope compatible-mode）。
- **图像生成**：火山方舟 **Seedream** `doubao-seedream-4-0-250828`（Bearer API Key，最多 10 张参考图，中文提示词）。

## 常用命令
```bash
# 后端测试（改 Go 代码后必跑）
go test -race ./...   &&   go vet ./...

# 本地开发：单服务（Go 托管前端 + API + /media）
cd web && npm run build && cd .. && \
  DB_PATH=/tmp/omc.db DATA_DIR=/tmp/omc_data PORT=8080 WEB_DIST=web/dist go build -o /tmp/omc ./cmd/server && /tmp/omc
# 或前后端分离热更新：go run ./cmd/server  +  cd web && npm run dev（Vite 代理 /api,/media）

# 前端构建校验（改前端后必跑）
cd web && npm run build

# Docker 部署到服务器（见 README「更新版本」）
docker buildx build --platform linux/amd64 -t oh-my-commic:latest --load .
```
> 运行需要 `.env`（`DASHSCOPE_API_KEY` + `ARK_API_KEY`）。config.Load 缺任一会 fatal。
> 用户管理相关 env（均可选）：`ADMIN_USERNAME`/`ADMIN_PASSWORD`/`ADMIN_EMAIL` 播种管理员（空用户名=不播种，非法即 fatal）；`INVITE_CODE` 指定全局邀请码（缺省则随机生成，启动时打日志 `邀请码: ...` 供分发）；注册赠送积分复用 `SIGNUP_CREDITS`（默认 100）。

## 架构 / 模块（`internal/`）
`models` 数据结构 · `config` 环境变量 · `db` SQLite+迁移 · `auth` 登录/session(持久化)/隔离中间件 · `book`/`asset`/`chapter`/`panel` CRUD · `community` 社区**跨表只读**(公开 feed 列表 + 公开阅读详情) + 点赞/独立访客浏览计数 · `comicify` 资产漫画化(Seedream 图生图) · `ai` 千问文本+两段式提示词+Seedream 生图客户端 · `render` 单格出图编排 · `story` 编排(storyboard-chat/process) · `imageutil` 参考图缩放 · `storage` 本地存图 · `httpx` 路由+SPA托管+请求日志。
前端关键：`AppShell`(应用壳=左栏 `SideNav`[Logo+社区/我的漫画 tab+底部 `SideNavUser` 用户区]+右侧 `<Outlet/>`；响应式) · `CommunityView`(社区内容区=`HeroBanner`+精选 `FeaturedRow`+`SortToggle`+`CommunityCard` 网格+空态) · `MyBooksView`(书架内容区，从 Bookshelf 抽出去顶栏) · `CommunityReader`(全屏公开阅读) · `ChatStoryboard`(第1段对话) · `StoryboardPanelCard`(单格可编辑) · `PanelGrid`/`PanelCard`(出图) · `BookReader`(翻页阅读) · `reader/BookReaderView`(私有/公开共用展示层)。

## 核心约定（务必遵守）
- **多用户隔离是第一要务**：每个数据访问都必须能追溯并校验 `user_id`；跨用户/不存在一律返回 **404**（不泄露存在性）。repository 查询带 `WHERE ... AND user_id=?`；资源按 panel→chapter→book→user 链式校验归属。
- **认证模型**：**用户名登录**（`{username,password}`，非邮箱）。注册走**邀请码闸门**——全局邀请码存 `settings` 表、可轮换，**空邀请码 = 注册未开放**（任何输入都拒，见 `auth/service.go` 的防御性判断）；仅 `role=admin` 可读/轮换邀请码（`GET|POST /api/v1/admin/invite-code[/rotate]`，非管理员 **403**）。角色 `role∈{admin,user}`：普通注册固定 `user`；管理员由**启动时 env 播种**（`ADMIN_USERNAME`/`ADMIN_PASSWORD`/`ADMIN_EMAIL`）——空用户名=不播种，用户名/密码**非法则启动 fatal**（`SeedAdmin` 幂等，已存在则跳过）。邮箱本期**收集但不验证**（deferred；字段/接口已留位）。
- **不可变**：service 返回新对象，不原地改入参。
- **小文件、单一职责**（200-400 行常态）。
- **错误处理**：`%w` 包裹；AI/上游错误按类映射（**429** 限流 / **504** 超时 / **502** 不可用，sentinel 在 `internal/ai/errors.go`，只看状态码/超时、不读 body）；**绝不把 API key 或上游 body 返回给客户端 / 打日志**。
- **用户数据最小化（隐私）**：**不长期存储用户原始上传图**（真人照片等比风格化形象更敏感）；资产"重新生成"对**当前锁定形象图**再漫画化，**不**引入 `source_url`、**不**保留原图。
- **社区公开只读（隐私）**：公开端点（`/api/v1/community/*`）走 `auth.OptionalUser`（可选读 session、**绝不 401**），严格 `is_public=1` 过滤——非公开/不存在一律 **404**（不泄露存在性）；作者**只**暴露 `nickname`+`avatarUrl`（**绝不** username/email）；公开阅读详情**不含**章节 `conversation`/`panelCount`。owner 发布/下架走 `PUT /api/v1/books/{id}/visibility`（`book.SetVisibility`：发布置 `is_public=1`+`published_at`，下架保留 `published_at`）；点赞端点走 `RequireUser`。
- **提示词**：只在 `internal/ai/prompts.go`、`internal/render/service.go`(stylePrefix+buildPrompt)、`internal/comicify/prompts.go`。改提示词看 `docs/ARCHITECTURE-AND-PROMPTS.md`。
- **迁移**：只用幂等 `ALTER TABLE ADD COLUMN`（`isDuplicateColumn` 容错），旧库不重建。
- **API 契约（前后端单一真相 = `docs/openapi.yaml`，OpenAPI 3.1）**：所有业务端点在 `/api/v1/*`（handler 用**资源相对路径** mount，版本前缀集中在 `internal/httpx/router.go` 的 `r.Route("/api/v1", ...)`）；`GET /api/health` **不**版本化。**改任何端点（路径/字段/状态码）必须同步更新 `docs/openapi.yaml`**——
  - **后端**：契约 E2E（`test/contract`，`kin-openapi` 校验每个真实响应）会挡住跑偏，跑在 `go test`/CI 里；
  - **前端**：`web/src/api`（`client.ts` 路径、`types.ts` 数据结构）也以 openapi.yaml 为准，两端共用同一契约；
  - 人读概览见 `docs/frontend-api.md`（只是概览，以 openapi.yaml 为准）。
- **前端路由**：`/` **重定向到 `/community`**（旧的两卡 `Home` 页已删除）；`AppShell` 应用壳（左栏 Logo+`🌈 社区`/`📚 我的漫画` tab+底部用户区）**包住** `/community`（公开 feed）与 `/my`（受保护书架，原在 `/`），右侧内容随 tab 切换；`/community/books/:id`=**全屏公开阅读器（不进壳）**；`/profile`、`/books/*`、`/chapters/*`、`/read/*`、`/login` 仍各自整页（沿用旧 `AppHeader`）；登录/注册成功跳 `/my`。每卡「公开/私密」开关 = `VisibilityToggle`。顶栏 `AppHeader`/左栏 `SideNav` 的 Logo 均指向 `/`（→ 重定向到公开社区 `/community`）——因 `AppHeader` 会随公开阅读器（`CommunityReader` 复用 `BookReaderView`）展示给**匿名访客**，Logo 必须指向公开路由，**不可**指向受保护的 `/my`（否则匿名读者会被弹去登录）。
- **git**：每功能一分支，完成后 `--no-ff` 合并回 main。提交信息中文 `type: 描述`。`.env`/`*.db`/`web/dist`/`node_modules` 均 gitignore，绝不入库/入镜像。

## 关键数据模型
`User(username, email, role, nickname=展示名, age, gender, avatar_url, credits)`（登录用 username；`role∈{admin,user}`）。全局邀请码存 `settings` 表。
`Book(user_id, cover_url, style, summary, is_public, like_count, view_count, published_at)` → `Character/Scene(book_id, image_url=锁定形象)` + `Chapter(book_id, order, status, is_cover, summary, conversation JSON, panel_count)` → `Panel(chapter_id, order, content, caption, character_ids, char_expressions, scene_id, event, location, image_prompt, image_url, status)`。
- **社区两表**：`book_likes(book_id, user_id)` PK 去重、`book_views(book_id, viewer_key)` PK 去重（viewer_key = 登录 `u:{id}` / 匿名 `anon:{clientId}`）；feed 索引 `idx_books_public_published`。
- 章节状态机：`draft→storyboarding→rendering→done`（放宽 + 同状态幂等，见 chapter/service.go）。
- **两段式分镜**：第1段只产出 `content`；第2段 `process` 从 content 解析出结构字段 + `image_prompt`；渲染用结构字段 + 参考图。

## 踩过的坑（避免重蹈）
- **表单双提交** → 用 `useRef` 同步守卫（`setState` 是异步，回车/双击会重复 POST 建重复数据）。
- **批量操作串行会"卡住"**：如"全部生成"应 `Promise.all` 并发，否则只有第一个在动。
- **多参考图必须显式绑定**："参考图1=角色X" 写进 prompt，否则模型张冠李戴。
- **分镜数**：prompt 要"恰好 N 格"硬指令，软措辞("大约")模型会只出 1 格。
- **LLM 的 id**：可能是数字/字符串/角色名 → `flexID` 容错、非法降级为 0 再过滤（否则整章解析 502）。
- **多格 JSON 截断**：`chatMaxTokens=8192` 防止长输出被截断成非法 JSON。
- **`go run` 会 fork 子进程**：kill 父进程不杀子进程会占端口；部署/本地重启用 `go build -o` 出二进制再跑。
- **SQLite FK 级联**：靠 DSN `_pragma=foreign_keys(1)`（每连接生效），不是单次 Exec。
- **交叉编译 Docker**：运行时阶段无 RUN（只 COPY/ENV），避免 arm64→amd64 需要 QEMU。

## 部署
Docker + docker-compose（`README.md` 有一键 demo 与更新流程）。服务器数据在具名卷 `omc-data:/data`（DB + 图片）。密钥经 compose `env_file: .env` 注入，不进镜像。

## 文档
- 提示词/模块详解：`docs/ARCHITECTURE-AND-PROMPTS.md`
- 设计文档（各功能）：`docs/superpowers/specs/`
- 运行/部署：`README.md`
