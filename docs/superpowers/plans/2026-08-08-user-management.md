# 用户管理模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** 把开放注册改为邀请码闸门 + 规范化账号（username 登录 / 强口令 / 唯一 email）+ env 播种 admin + 用户 Profile（昵称/年龄/性别/头像）+ admin 查看/轮换邀请码。

**Architecture:** 沿用 `internal/auth` 分层（handler→service→repo）。`users` 表 `nickname` 由登录名改展示名，新增 `username/email/role/age/gender/avatar_url` + `settings` 表存全局邀请码。注册加邀请码校验 + 强校验，登录改 username，新增 `RequireAdmin` 与 admin 邀请码端点。启动时按 env 幂等播种 admin + 邀请码。全端点走 `/api/v1`，契约进 `docs/openapi.yaml`。

**Tech Stack:** Go（chi、modernc sqlite、bcrypt、net/mail）、React+Vite+TS、kin-openapi 契约测试、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-08-08-user-management-design.md`（本 plan 的真相源）。

## Global Constraints
- 多用户隔离：数据访问带 `user_id` 校验，跨用户/不存在 → 404。
- 不可变：service 返回新对象，不原地改入参。错误 `%w` 包裹；绝不泄露 hash/邀请码(非 admin)/上游细节。
- API 契约单一真相 = `docs/openapi.yaml`；改任何 `/api/v1/*` 必须同步它，否则 `test/contract` 红。
- 迁移只用幂等 `ALTER TABLE ADD COLUMN` + `CREATE ... IF NOT EXISTS`；旧库不重建（demo 库可重置）。
- 提交信息中文 `type: 描述`，不加 Co-Authored-By。测试全 mock、不真发信、不碰真实 key、无 env 也能过。
- username `^[a-z][a-z0-9_]{2,19}$`；password 8–64 ASCII 可见、含字母+数字、无空格；email 用 `net/mail.ParseAddress` 且小写唯一；nickname 1–30 去空白、非唯一；age 0–150；gender ∈ {男,女,其他,''}。

---

## 文件结构（谁负责什么）
**后端**
- `internal/config/config.go` — 新增 `AdminUsername/AdminPassword/AdminEmail/InviteCode`。
- `internal/models/models.go` — `User` 增 `Username/Email/Role/Age/Gender/AvatarURL`。
- `internal/db/migrate.go` — users 新列 + 唯一索引 + `settings` 表。
- `internal/auth/validate.go`（新）— 校验器。
- `internal/auth/repo.go` — Create 新签名、`ByUsername`、`ByID`、`UpdateProfile`、`SetAvatar`；扫描新列。
- `internal/auth/invite.go`（新）— `InviteRepo`（settings 表 get/set/rotate/seed）。
- `internal/auth/service.go` — Register/Login/Me/UpdateProfile/SeedAdmin/Invite。
- `internal/auth/middleware.go` — 新增 `RequireAdmin`。
- `internal/auth/handler.go` — 端点更新 + 新端点；`MountProtected`/新增 `MountAdmin`。
- `internal/httpx/router.go` — 挂 profile/avatar（保护）+ admin 组（RequireAdmin）。
- `cmd/server/main.go` — 启动播种 admin + 邀请码；wiring。
- `docs/openapi.yaml`、`test/contract/contract_test.go` — 契约同步 + E2E。

**前端**
- `web/src/api/types.ts`、`web/src/api/client.ts`、`web/src/auth/useAuth.tsx`、`web/src/pages/Login.tsx`、`web/src/pages/Profile.tsx`（新）、`web/src/App.tsx`（路由）、`web/src/components/AppHeader.tsx`。
- `web/src/**/*.test.ts(x)`（Vitest）、`web/e2e/*.spec.ts`（Playwright）、`.github/workflows/e2e.yml`。

**文档**：`docs/frontend-api.md`、`CLAUDE.md`。

---

## Task 1: 配置 — admin/邀请码 env

**Files:** Modify `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces — Produces:** `Config` 增字段 `AdminUsername, AdminPassword, AdminEmail, InviteCode string`。

- [ ] **Step 1: 失败测试** — 在 `config_test.go` 加：设 `ADMIN_USERNAME=admin ADMIN_PASSWORD=Passw0rd ADMIN_EMAIL=a@b.com INVITE_CODE=welcome123`，`Load()` 后断言四字段被读入；不设时为空串。
- [ ] **Step 2:** `go test ./internal/config/` → FAIL（字段不存在）。
- [ ] **Step 3: 实现** — `Config` 加四字段；`Load` 里 `AdminUsername: os.Getenv("ADMIN_USERNAME")` 等（全部 `os.Getenv`，**均非致命缺省**，不影响现有 DASHSCOPE/ARK 必填）。
- [ ] **Step 4:** `go test ./internal/config/` → PASS。
- [ ] **Step 5:** commit `feat: config 新增 admin 播种与邀请码 env`。

---

## Task 2: 数据模型 + 迁移

**Files:** Modify `internal/models/models.go`, `internal/db/migrate.go`; Test `internal/db/migrate_test.go`

**Interfaces — Produces:** `models.User` 增 `Username string json:"username"`, `Email string json:"email"`, `Role string json:"role"`, `Age int json:"age"`, `Gender string json:"gender"`, `AvatarURL string json:"avatarUrl"`（`Nickname/Credits` 已存在；`PasswordHash json:"-"`）。settings 表 `(key TEXT PRIMARY KEY, value TEXT NOT NULL)`。

- [ ] **Step 1: 失败测试** — `migrate_test.go`：打开临时库跑 `Migrate` 两次（幂等）；断言 `users` 有列 `username,email,role,age,gender,avatar_url`（`PRAGMA table_info(users)`），存在唯一索引 `idx_users_username`/`idx_users_email`，存在表 `settings`。
- [ ] **Step 2:** `go test ./internal/db/` → FAIL。
- [ ] **Step 3: 实现** —
  - `models.User` 加上述字段。
  - `migrate.go`：把 `users` 的 `CREATE TABLE IF NOT EXISTS` 改为新结构（含新列；`nickname TEXT NOT NULL DEFAULT ''` **去掉 UNIQUE**）。
  - `alterStatements` 追加（幂等，`isDuplicateColumn` 容错）：
    ```go
    `ALTER TABLE users ADD COLUMN username TEXT NOT NULL DEFAULT ''`,
    `ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`,
    `ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`,
    `ALTER TABLE users ADD COLUMN age INTEGER NOT NULL DEFAULT 0`,
    `ALTER TABLE users ADD COLUMN gender TEXT NOT NULL DEFAULT ''`,
    `ALTER TABLE users ADD COLUMN avatar_url TEXT NOT NULL DEFAULT ''`,
    ```
  - 追加 `schemaStatements`（或独立 exec）：
    ```go
    `CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '')`,
    ```
  - 迁移末尾建唯一索引（幂等）：
    ```go
    `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username <> ''`,
    `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email <> ''`,
    ```
    （放一个新的 `indexStatements` 切片，在 alter 之后执行。）
- [ ] **Step 4:** `go test ./internal/db/` → PASS（幂等两次也过）。
- [ ] **Step 5:** commit `feat: users 增用户名/邮箱/角色/资料列 + settings 表 + 唯一索引`。

---

## Task 3: 校验器 `validate.go`

**Files:** Create `internal/auth/validate.go`, `internal/auth/validate_test.go`

**Interfaces — Produces（供 service 用）:**
```go
var (
    ErrBadUsername = errors.New("用户名需 3-20 位，小写字母开头，仅含小写字母/数字/下划线")
    ErrBadPassword = errors.New("密码需 8-64 位，且同时包含字母和数字，不能有空格或中文")
    ErrBadEmail    = errors.New("邮箱格式不正确")
    ErrBadNickname = errors.New("昵称需 1-30 个字符")
    ErrBadAge      = errors.New("年龄需在 0-150 之间")
    ErrBadGender   = errors.New("性别只能是 男/女/其他 或留空")
)
func ValidateUsername(s string) error
func ValidatePassword(s string) error
func NormalizeEmail(s string) (string, error) // 校验 + 小写化 + 去空白
func NormalizeNickname(s, fallback string) (string, error) // 去空白；空则用 fallback(username)
func ValidateAge(n int) error
func ValidateGender(s string) error
```

- [ ] **Step 1: 失败测试（表驱动）** — 覆盖：username `ab`(短)/`Abc`(大写)/`1ab`(数字开头)/`a b`(空格)/`中文`/合法 `a_1`;  password `short1`/`nodigitsss`/`12345678`(无字母)/`has space1`/合法 `abcd1234`; email `x`/`a@b`(可接受?用 ParseAddress)/合法 `a@b.com` 且断言小写化; nickname `''`→fallback、31 长→错、`  hi  `→`hi`; age -1/151/0/30; gender `男`/`x`/``。
- [ ] **Step 2:** `go test ./internal/auth/ -run Validate` → FAIL。
- [ ] **Step 3: 实现** — 用 `regexp.MustCompile(\`^[a-z][a-z0-9_]{2,19}$\`)`；password 遍历 rune 判 ASCII 可见(33–126)且统计有无字母/数字、拒空格；`NormalizeEmail` 用 `net/mail.ParseAddress` 后取 `addr.Address` 再 `strings.ToLower`；nickname `strings.TrimSpace` + rune 计数 1–30，空则 fallback；gender `switch`。
- [ ] **Step 4:** `go test ./internal/auth/ -run Validate` → PASS。
- [ ] **Step 5:** commit `feat: auth 校验器(用户名/密码/邮箱/昵称/年龄/性别)`。

---

## Task 4: User repo 改造

**Files:** Modify `internal/auth/repo.go`, `internal/auth/repo_test.go`

**Interfaces:**
- Consumes: `models.User`（Task 2）。
- Produces:
```go
// 建号（注册/播种共用）
func (r *UserRepo) Create(u NewUser) (models.User, error)
type NewUser struct{ Username, Email, PasswordHash, Nickname, Role string; Credits int }
func (r *UserRepo) ByUsername(username string) (models.User, error)
func (r *UserRepo) ByID(id int64) (models.User, error) // 已存在，扩展扫描新列
func (r *UserRepo) UpdateProfile(userID int64, nickname string, age int, gender string) (models.User, error)
func (r *UserRepo) SetAvatar(userID int64, avatarURL string) (models.User, error)
// 唯一冲突辨识
var ErrUsernameTaken = errors.New("用户名已被占用")
var ErrEmailTaken = errors.New("邮箱已注册")
// 保留 Spend/Refund/Credits 不变
```
唯一冲突：`Create` 捕获 sqlite `UNIQUE constraint failed: ...idx_users_username`/`...username`、`...email`，映射 `ErrUsernameTaken`/`ErrEmailTaken`（用 `strings.Contains(err.Error(), "username")` 等辨识）。删除旧 `ByNickname`（登录改 username）与旧 `Create(nickname,hash,credits)`（更新所有调用点：service、cmd/server、测试）。

- [ ] **Step 1: 失败测试** — repo_test：`Create` 两个不同用户成功、含 credits/role/nickname 回读；同 username 二次 `Create` → `ErrUsernameTaken`；同 email → `ErrEmailTaken`；`ByUsername` 命中/未命中；`UpdateProfile` 改昵称/年龄/性别回读；`SetAvatar` 回读；`Spend` 边界仍过。
- [ ] **Step 2:** `go test ./internal/auth/ -run Repo` → FAIL。
- [ ] **Step 3: 实现** — 改所有 SELECT 列为 `id,username,email,password_hash,role,nickname,age,gender,avatar_url,credits,created_at`，`scanUser` 同步；`Create` INSERT 这些列；`UpdateProfile`/`SetAvatar` 用 `UPDATE ... WHERE id=?` 后回读 `ByID`。
- [ ] **Step 4:** `go test ./internal/auth/ -run Repo` → PASS。
- [ ] **Step 5:** commit `feat: user repo 支持 username/email 唯一、profile、头像`。

---

## Task 5: 邀请码 repo

**Files:** Create `internal/auth/invite.go`, `internal/auth/invite_test.go`

**Interfaces — Produces:**
```go
type InviteRepo struct{ db *sql.DB }
func NewInviteRepo(d *sql.DB) *InviteRepo
func (r *InviteRepo) Get() (string, error)                 // 读 settings['invite_code']；无则 ""
func (r *InviteRepo) Set(code string) error                // upsert
func (r *InviteRepo) Rotate() (string, error)              // 生成新码(randomCode) 写入并返回
func (r *InviteRepo) Seed(preferred string) (string, error)// 若已存在返回现值；否则用 preferred 或 randomCode 写入并返回
func randomCode() string // 10 位 [a-z0-9]，crypto/rand
```
`Set` 用 `INSERT INTO settings(key,value) VALUES('invite_code',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`。

- [ ] **Step 1: 失败测试** — `Get` 空库返回 ""；`Seed("welcome")` 返回 "welcome"，再 `Seed("other")` 仍返回 "welcome"（幂等）；`Rotate` 返回新 10 位码且与旧不同、`Get` 与之一致；`randomCode` 长度 10、字符集合法。
- [ ] **Step 2:** `go test ./internal/auth/ -run Invite` → FAIL。
- [ ] **Step 3: 实现** — `crypto/rand` 取字节映射到 `abcdefghijklmnopqrstuvwxyz0123456789`。
- [ ] **Step 4:** PASS。
- [ ] **Step 5:** commit `feat: 全局邀请码 repo(settings 表, seed/get/rotate)`。

---

## Task 6: Auth service

**Files:** Modify `internal/auth/service.go`, `internal/auth/service_test.go`

**Interfaces:**
- Consumes: `UserRepo`(T4)、`InviteRepo`(T5)、validators(T3)、`Session`。
- Produces:
```go
func NewService(repo *UserRepo, invites *InviteRepo, sess *Session, signupCredits int) *Service
var ErrBadInvite = errors.New("邀请码不正确")
type RegisterInput struct{ Username, Password, Email, InviteCode, Nickname string }
func (s *Service) Register(in RegisterInput) (token string, u models.User, err error) // 校验+唯一+建号(role=user)+自动登录发 token
func (s *Service) Login(username, password string) (token string, u models.User, err error) // ErrInvalidCredentials
func (s *Service) Me(userID int64) (models.User, error) // 已有，返回全 profile
func (s *Service) UpdateProfile(userID int64, nickname string, age int, gender string) (models.User, error)
func (s *Service) SetAvatar(userID int64, url string) (models.User, error)
func (s *Service) SeedAdmin(username, password, email string, credits int) error // 幂等：空 username 跳过；username 非法/密码非法→返回 error(启动 fatal)；已存在跳过；否则建 role=admin
func (s *Service) InviteCode() (string, error)      // admin 用
func (s *Service) RotateInvite() (string, error)    // admin 用
func (s *Service) SeedInvite(preferred string) (string, error)
```
Register 顺序：`invites.Get()` 比对 `in.InviteCode`（不等→`ErrBadInvite`）→ `ValidateUsername/ValidatePassword/NormalizeEmail/NormalizeNickname(fallback=username)` → bcrypt → `repo.Create(NewUser{...Role:"user",Credits:signupCredits})`（唯一冲突透传 `ErrUsernameTaken/ErrEmailTaken`）→ `sess.Issue(u.ID)`。

- [ ] **Step 1: 失败测试** — service_test（用真实 sqlite + repos）：Register 成功返回非空 token 且 `u.Role=="user"`、credits=signup、nickname 缺省=username；错邀请码→`ErrBadInvite`；弱密码→`ErrBadPassword`；重复 username→`ErrUsernameTaken`；Login 正确/错误(→`ErrInvalidCredentials`)；`UpdateProfile` 生效；`SeedAdmin` 建 admin、二次幂等跳过、空 username 跳过、非法 username 返回 error。
- [ ] **Step 2:** `go test ./internal/auth/ -run Service` → FAIL。
- [ ] **Step 3: 实现** 如上。`SeedAdmin`：`username==""`→nil；`ValidateUsername/Password` 失败→return err；`repo.ByUsername` 命中→nil(跳过)；否则 bcrypt+`Create{Role:"admin"}`。
- [ ] **Step 4:** PASS。
- [ ] **Step 5:** commit `feat: auth service(邀请码注册/username登录/profile/admin播种/邀请码)`。

---

## Task 7: RequireAdmin 中间件

**Files:** Modify `internal/auth/middleware.go`, `internal/auth/middleware_test.go`

**Interfaces:**
- Consumes: 现有 `RequireUser`、context user id、`UserRepo.ByID`(取 role)。
- Produces: `func RequireAdmin(sess *Session, repo *UserRepo) func(http.Handler) http.Handler`（先 RequireUser 语义拿 userID，再 `repo.ByID` 校验 `role=="admin"`，否则 403 JSON `{"error":"需要管理员权限"}`）。

- [ ] **Step 1: 失败测试** — 组装 chi + RequireAdmin 包一个返回 200 的 handler：admin session→200；普通用户 session→403；无 session→401。
- [ ] **Step 2:** FAIL。
- [ ] **Step 3: 实现** — 复用 `RequireUser` 链：可在其内部再查 role。注意：`RequireAdmin` 需要 session 已解析——实现为「先跑 RequireUser 的解析逻辑，再查 role」。最简：`RequireAdmin` 返回的中间件里手动解析 token(同 RequireUser)→取 userID→`repo.ByID`→role 判断。抽公共 `resolveUser` 避免重复。
- [ ] **Step 4:** PASS。
- [ ] **Step 5:** commit `feat: RequireAdmin 中间件(仅管理员)`。

---

## Task 8: Handlers + 路由

**Files:** Modify `internal/auth/handler.go`, `internal/httpx/router.go`; Modify tests `internal/auth/handler_test.go`, `internal/httpx/router_test.go`

**Interfaces — Produces（挂在 `/api/v1` 组，handler 用资源相对路径）:**
- `POST /register` `{username,password,email,inviteCode,nickname?}` → 200 用户(+set cookie)；映射 `ErrBadInvite/ErrBad*`→400、`ErrUsernameTaken/ErrEmailTaken`→409。
- `POST /login` `{username,password}` → 200(+cookie)；`ErrInvalidCredentials`→401。
- `GET /me`（保护，已存在）→ 全 profile。
- `PUT /me/profile` `{nickname,age,gender}` → 200 用户；校验错→400。
- `POST /me/avatar`（保护，multipart `file`）→ 存 `storage.Local` 到 `users/{userID}/`，`SetAvatar`→200 用户；类型/大小错→400。
- `GET /admin/invite-code`（admin）→ `{inviteCode}`；`POST /admin/invite-code/rotate`（admin）→ `{inviteCode}`。
- `Handler` 增 `MountAdmin(r)`；router 里新增 `v1.Group` with `RequireAdmin` 挂 admin 路由；`/me/profile`、`/me/avatar` 进现有保护组。头像上传复用 asset 的图片嗅探常量思路（png/jpg/webp，≤2MiB）。

- [ ] **Step 1: 失败测试** — handler_test：注册成功返回 200+含 cookie、错邀请码 400、重复 username 409；登录 username 成功/401；`PUT /me/profile` 改昵称成功、非法 gender 400；`POST /me/avatar` 传 png 成功回填 avatarUrl、传 txt 400；admin：普通用户 `GET /api/v1/admin/invite-code`→403、admin→200 且含码、rotate 后码变化。router_test：新路由 200/403 正确。
- [ ] **Step 2:** `go test ./internal/auth/ ./internal/httpx/` → FAIL。
- [ ] **Step 3: 实现** handler + 路由挂载；头像存储用注入的 `storage.Local`（handler 增字段，参考 asset handler 的上传/嗅探）。
- [ ] **Step 4:** PASS。
- [ ] **Step 5:** commit `feat: 注册(邀请码)/登录(username)/profile/头像/admin邀请码 端点`。

---

## Task 9: 启动 wiring（播种 + 挂载）

**Files:** Modify `cmd/server/main.go`

**Interfaces:** Consumes T5/T6/T8。

- [ ] **Step 1:** （无独立单测；靠编译 + 契约 E2E 覆盖。）人工核对：`main.go` 里
  - `inviteRepo := auth.NewInviteRepo(d)`；`authSvc := auth.NewService(userRepo, inviteRepo, sess, cfg.SignupCredits)`。
  - migrate 后：`code, _ := authSvc.SeedInvite(cfg.InviteCode); log.Printf("邀请码: %s", code)`；`if err := authSvc.SeedAdmin(cfg.AdminUsername, cfg.AdminPassword, cfg.AdminEmail, cfg.SignupCredits); err != nil { log.Fatalf(...) }`。
  - auth handler 注入 `storage.Local`（头像）；router `Deps` 传入 admin 中间件所需 `*auth.UserRepo`。
- [ ] **Step 2:** `go build ./...` → 通过。
- [ ] **Step 3:** `CGO_ENABLED=1 go test -race ./...` → 现有全过（新端点契约在 Task 10）。
- [ ] **Step 4:** commit `feat: 启动播种 admin+邀请码, 挂载用户管理路由`。

---

## Task 10（契约任务）: OpenAPI + 契约 E2E

**Files:** Modify `docs/openapi.yaml`, `test/contract/contract_test.go`

- [ ] **Step 1: 失败测试** — 在 `contract_test.go` 用新流程：`request-code` 无 → 直接 `POST /api/v1/register`（带 seed 的邀请码——测试内 `SeedInvite("welcome123")` 或从 service 取）→ 200 校验 `User` schema（含 username/email/role/nickname/age/gender/avatarUrl/credits）；`GET /me`；`PUT /me/profile` 200；错邀请码 400；admin 端点用播种 admin 登录后 200、普通用户 403。每步 `openapi3filter.ValidateResponse`。先跑 → FAIL（openapi 未含新端点/字段）。
- [ ] **Step 2: 实现** `docs/openapi.yaml`：
  - `User` schema 加 `username,email,role,age,gender,avatarUrl`（`nickname` 保留为展示名）。
  - `register` 请求体改 `{username,password,email,inviteCode,nickname?}`，响应 200 User，新增 409。
  - `login` 请求体改 `{username,password}`。
  - 新增 `PUT /api/v1/me/profile`、`POST /api/v1/me/avatar`(multipart)、`GET /api/v1/admin/invite-code`、`POST /api/v1/admin/invite-code/rotate`（响应 `{inviteCode}` schema），补 403。
- [ ] **Step 3:** `CGO_ENABLED=1 go test ./test/contract/` → PASS（负向：临时改 openapi 去掉 role required 应变红以证真校验）。
- [ ] **Step 4:** commit `docs+test: openapi 契约同步用户管理端点 + 契约 E2E`。

---

## Task 11: 前端 types + client + useAuth

**Files:** Modify `web/src/api/types.ts`, `web/src/api/client.ts`, `web/src/auth/useAuth.tsx`

**Interfaces — Produces:**
```ts
// types.ts
type User = { id:number; username:string; email:string; role:'admin'|'user'; nickname:string; age:number; gender:string; avatarUrl:string; credits:number; createdAt:string }
// client.ts
api.register(body:{username;password;email;inviteCode;nickname?}) // POST /api/v1/register
api.login(body:{username;password})                                // POST /api/v1/login
api.updateProfile(body:{nickname;age;gender})                      // PUT /api/v1/me/profile
api.uploadAvatar(file:File)                                        // POST /api/v1/me/avatar (multipart)
api.getInviteCode() / api.rotateInviteCode()                       // admin
// useAuth: register(input)->自动登录; login(username,password)
```

- [ ] **Step 1: 失败测试** — Vitest `useAuth`/client 轻测：mock fetch，`api.register` 打到 `/api/v1/register` 且带 body；`User` 类型编译期即可（tsc）。（重点是 tsc 通过 + 关键调用路径。）
- [ ] **Step 2:** `cd web && npm run test` → 失败/`npm run build` tsc 报错。
- [ ] **Step 3: 实现** 更新类型与 client 方法；`useAuth` 的 `register` 调 `/register`（返回已 set cookie）再 `setUser`。
- [ ] **Step 4:** `npm run test` + `npm run build` → PASS。
- [ ] **Step 5:** commit `feat(web): User 类型/邀请码注册/username 登录/profile client`。

---

## Task 12: 登录/注册页

**Files:** Modify `web/src/pages/Login.tsx`; Test `web/src/pages/Login.test.tsx`（可选轻测）

- [ ] **Step 1:** 注册表单字段：username、password、email、inviteCode、nickname(选填)；登录表单：username、password。前端用与后端一致的正则给即时提示（username/password），提交仍以后端 400/409 文案为准（`errorMessage`）。沿用 `useSubmitOnce` 防重。
- [ ] **Step 2:** `npm run build` 通过（tsc）。可加一条 Vitest：渲染注册 tab 出现"邀请码"输入。
- [ ] **Step 3:** commit `feat(web): 登录用 username、注册加邮箱+邀请码`。

---

## Task 13: Profile 页 + 头像 + admin 邀请码面板

**Files:** Create `web/src/pages/Profile.tsx`; Modify `web/src/App.tsx`(路由 `/profile`), `web/src/components/AppHeader.tsx`

- [ ] **Step 1:** Profile 页：昵称/年龄/性别表单（`api.updateProfile`）+ 头像上传（`api.uploadAvatar`，成功后 `refreshUser`）+ **仅 `user.role==='admin'`** 显示"邀请码"卡片（`getInviteCode` 展示 + `rotateInviteCode` 轮换按钮）。AppHeader：头像有 `avatarUrl` 显示图、否则昵称首字；菜单加"个人资料"跳 `/profile`。
- [ ] **Step 2:** `npm run build` 通过。Vitest：给 Profile 传 admin/user 两种 user，断言邀请码卡片显隐。
- [ ] **Step 3:** commit `feat(web): 个人资料页(资料/头像)+admin 邀请码面板`。

---

## Task 14（测试+CI 任务）: 前端单测 + E2E + CI

**Files:** Add/He `web/src/**/*.test.tsx`; Modify `web/e2e/smoke.spec.ts` 或新增 `web/e2e/register.spec.ts`; Modify `.github/workflows/e2e.yml`

- [ ] **Step 1:** Vitest 汇总：validators 提示、Profile 邀请码显隐、useSubmitOnce 仍过（已存在）。
- [ ] **Step 2:** Playwright E2E 新增「邀请码注册 → 登录 → 改昵称」：用固定 `INVITE_CODE`（见下）注册唯一 username → 落书架 → 进 `/profile` 改昵称 → 断言 header 显示新昵称。**不碰 AI/邮件**。
- [ ] **Step 3:** `.github/workflows/e2e.yml` 启动服务的步骤加 env：`INVITE_CODE=welcome123 ADMIN_USERNAME=admin ADMIN_PASSWORD=Passw0rd1`（假值），使注册可用。本地按同样 env 实跑 `npx playwright test` 通过。
- [ ] **Step 4:** `npm run test` + 本地 E2E 通过；`python3 -c "import yaml;yaml.safe_load(open('.github/workflows/e2e.yml'))"`。
- [ ] **Step 5:** commit `test(web): profile/邀请码单测 + 邀请码注册 E2E + e2e.yml env`。

---

## Task 15（文档任务）: frontend-api.md + CLAUDE.md

**Files:** Modify `docs/frontend-api.md`, `CLAUDE.md`

- [ ] **Step 1:** `frontend-api.md`：端点速查加 `register(新体)`、`login(username)`、`PUT /me/profile`、`POST /me/avatar`、`GET|POST /admin/invite-code(...)`；User 结构指向 openapi。
- [ ] **Step 2:** `CLAUDE.md` 核心约定：把认证模型更新为「**username 登录 + 邀请码闸门注册 + env 播种 admin(ADMIN_USERNAME/PASSWORD/EMAIL) + role(admin/user) + 邀请码 settings 表可轮换**；邮箱本期不验证(deferred)」；`常用命令`/env 说明补 `ADMIN_*`、`INVITE_CODE`。
- [ ] **Step 3:** commit `docs: 用户管理认证模型写入 CLAUDE.md 与 API 概览`。

---

## Global 验收（全部任务后）
- `CGO_ENABLED=1 go test -race ./...` + `go vet ./...` 全绿（含 `test/contract`）。
- `cd web && npm run test && npm run build` 通过；本地 Playwright（带 INVITE_CODE/ADMIN_* env）通过。
- 手动冒烟：重置本地库 → 启动带 `ADMIN_USERNAME/PASSWORD` + `INVITE_CODE` → admin 登录看邀请码 → 用邀请码注册普通用户 → 改 profile/传头像。
