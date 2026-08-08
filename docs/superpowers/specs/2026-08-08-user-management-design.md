# 用户管理模块设计（注册防护 / 规范 / Profile / Admin）

> 日期 2026-08-08。目标：把"任何人都能注册、会烧爆付费 API 额度"的开放注册，改为**邀请码闸门 + 规范化账号 + env 播种 admin**，并支持用户 Profile（昵称/年龄/性别/头像）。
> 本 spec 经 brainstorming 对齐，作为 writing-plans 的输入。

## 1. 目标与非目标

**目标**
- 注册需**邀请码**（全局单一、可轮换），挡住开放注册，保住 DashScope/Ark 额度。
- **username / password / email** 规范化校验，避免脏数据。
- **email** 注册必填、唯一、存库（**本期不验证**）。
- 用户 **Profile**：昵称（展示名）、年龄、性别、头像。
- **admin** 由**环境变量播种**（非"首个注册用户"），仅 admin 可查看/轮换邀请码。
- 交付必须**同步**：OpenAPI 契约、文档、测试、CI。

**非目标（YAGNI，留接口/字段，后续再加）**
- 邮箱验证码 + 真实邮件发送（留 `email` 字段与 `EmailSender` 抽象位）。
- 密码找回、OAuth、admin 用户列表/封禁。

## 2. 决策基线（已与用户确认）
- 邮件：**暂不发送、无验证码**（对接云厂商成本高，后续增强）。
- 邀请码：**全局单一、可轮换**，仅 admin 可见。
- admin：**env 播种**（`ADMIN_USERNAME`/`ADMIN_PASSWORD`/`ADMIN_EMAIL`），启动时若不存在则创建；普通注册一律 `role=user`。
- 数据：**可重置**（本地 demo 库），允许结构调整、丢弃旧 nickname 登录账号。
- **username 与 email 各建唯一索引**；nickname 展示名非唯一。

## 3. 数据模型

### 3.1 `users`（重构；`nickname` 从登录名→展示名）
| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | INTEGER PK | |
| `username` | TEXT | **登录名**，唯一索引，格式见 §4 |
| `email` | TEXT | 注册必填，唯一索引，本期不验证 |
| `password_hash` | TEXT | bcrypt |
| `role` | TEXT | `admin`/`user`，默认 `user` |
| `nickname` | TEXT | 展示名，可中文/空格，**非唯一** |
| `age` | INTEGER | 0=未设，0–150 |
| `gender` | TEXT | `男`/`女`/`其他`/`''` |
| `avatar_url` | TEXT | `/media/users/{id}/...`，可空 |
| `credits` | INTEGER | 沿用（默认 100） |
| `created_at` | TEXT | 沿用 |

### 3.2 `settings`
`key TEXT PRIMARY KEY, value TEXT`。存全局邀请码：`key='invite_code'`。启动播种：若无该行，则取 env `INVITE_CODE`，否则随机生成 8–12 位并**打服务器日志**（供 ops 首次分发）。

### 3.3 迁移策略（遵循"幂等 ADD COLUMN + 不重建旧库"，数据可重置）
- 修改**新建**（`CREATE TABLE IF NOT EXISTS users`）为新结构：`username`、`email`、`role`、`nickname`（去 UNIQUE）、`age`、`gender`、`avatar_url`、`credits`。
- 对既有库：幂等 `ALTER TABLE users ADD COLUMN ...` 补齐 `username/email/role/age/gender/avatar_url`（`nickname/credits` 已存在）。
- `CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username <> ''`；email 同理（partial index，避免既有空值冲突）。
- `CREATE TABLE IF NOT EXISTS settings (...)`。
- **本地/demo 库直接重置**（删 `*.db`）取最干净结构；线上具名卷同理按需重置。

## 4. 校验规范（系统边界强校验，fail fast）
- **username**：`^[a-z][a-z0-9_]{2,19}$`（小写字母开头，仅小写字母/数字/下划线，3–20 位；无空格/中文/大写）。
- **password**：8–64 位，仅 ASCII 可见字符，**至少 1 字母 + 1 数字**，无空格/中文/控制字符。
- **email**：`net/mail.ParseAddress` 校验；小写化后唯一。
- **nickname**（展示）：去首尾空格后 1–30 字符，允许中文/空格，禁控制字符；缺省用 username。
- **age**：0–150（0=未设）。**gender**：仅 `男`/`女`/`其他`/`''`。
- 校验放 `internal/auth`（或 `internal/auth/validate.go`），有独立表驱动单测。

## 5. 注册 / 登录 / 会话

### 5.1 注册 `POST /api/v1/register`
入参 `{username, password, email, inviteCode, nickname?}`。流程：
1. 校验邀请码 == 当前 `settings.invite_code`，否则 **400**「邀请码不正确」。
2. 校验 username/password/email/nickname 格式，任一不合 **400**（各自友好文案）。
3. `username`/`email` 唯一，冲突 **409**（「用户名已被占用」/「邮箱已注册」）。
4. bcrypt 存密码，建用户 `role=user`、发初始积分、`nickname` 缺省=username。
5. **自动登录**：签发 session cookie，返回当前用户（含 profile/role，不含 password/hash）。

### 5.2 登录 `POST /api/v1/login`
改为 `{username, password}`（不再用 nickname）。失败统一 **401** `ErrInvalidCredentials`（不区分用户名不存在/密码错）。

### 5.3 会话
沿用现有 `sessions` 表 + `RequireUser` 中间件。新增 `RequireAdmin` = `RequireUser` + `role=='admin'`，否则 **403**。

## 6. Admin（env 播种）
- 配置：`ADMIN_USERNAME`、`ADMIN_PASSWORD`（成对必填才播种）、`ADMIN_EMAIL`（可选，默认占位如 `admin@local`）。
- 启动（migrate 后）：若 `ADMIN_USERNAME` 已设置：
  - 先按 §4 校验 `ADMIN_USERNAME`/`ADMIN_PASSWORD`，不合规范 → **fatal**（逼 ops 改对）。
  - 若该 username 不存在 → 创建 admin（bcrypt、`role=admin`、发初始积分、`email=ADMIN_EMAIL`）；**已存在则跳过、不覆盖**（幂等）。日志记录 seeded/exists（不打印密码）。
- 邀请码端点（仅 admin）：
  - `GET /api/v1/admin/invite-code` → `{ "inviteCode": "..." }`
  - `POST /api/v1/admin/invite-code/rotate` → 生成新码、写库、返回新码。
- 前端：仅 `role==='admin'` 渲染"邀请码"面板（查看 + 轮换）；非 admin 请求 **403**。

## 7. Profile & 头像
- `GET /api/v1/me` → 返回 `{id, username, email, role, nickname, age, gender, avatarUrl, credits, createdAt}`（永不含 hash）。
- `PUT /api/v1/me/profile` → `{nickname, age, gender}`（校验后更新，不可变返回新对象）。
- `POST /api/v1/me/avatar`（multipart `file`）→ 复用上传/存储（`storage.Local`），**用户维度目录** `/media/users/{userId}/`，回填 `avatar_url`；限图片类型 + ≤2MiB。
- 前端新增 **Profile 页** `/profile`：昵称/年龄/性别表单 + 头像上传；顶栏头像有 `avatarUrl` 则显示，否则昵称首字。

## 8. API 端点清单（v1，全部进 `docs/openapi.yaml`）
| 方法 路径 | 说明 | 鉴权 |
|---|---|---|
| `POST /api/v1/register` | 邀请码注册（改造）| 公开 |
| `POST /api/v1/login` | username 登录（改造）| 公开 |
| `POST /api/v1/logout` | 退出 | 公开 |
| `GET /api/v1/me` | 当前用户（含 profile/role）| 用户 |
| `PUT /api/v1/me/profile` | 改昵称/年龄/性别 | 用户 |
| `POST /api/v1/me/avatar` | 上传头像 | 用户 |
| `GET /api/v1/admin/invite-code` | 查看邀请码 | admin |
| `POST /api/v1/admin/invite-code/rotate` | 轮换邀请码 | admin |

## 9. 错误处理
- 校验类 **400**（用户名/密码/邮箱/昵称/年龄/性别 各自友好中文）。
- 唯一冲突 **409**（用户名/邮箱已占用）。
- 邀请码错 **400**。登录失败 **401**。非 admin 访问 admin 端点 **403**。
- 沿用：跨用户/不存在 **404**、AI 上游 429/504/502。
- 绝不泄露 password hash、邀请码（除 admin 端点）、内部细节。

## 10. 配置新增（`internal/config`）
- `ADMIN_USERNAME` / `ADMIN_PASSWORD` / `ADMIN_EMAIL`（admin 播种；缺省不播种）。
- `INVITE_CODE`（初始邀请码；缺省随机生成 + 日志）。
- 复用 `SIGNUP_CREDITS`。这些均**非致命**缺省（除 admin username 不合规范时 fatal）。

## 11. 交付与维护要求（**plan 必须包含独立任务**）
> 每个功能改动都要"带着"以下四类维护，不是可选：
1. **契约(API)**：改任何 `/api/v1/*` 必须同步 `docs/openapi.yaml`；后端契约 E2E（`test/contract`，kin-openapi 校验真实响应）覆盖新端点。
2. **文档**：`docs/frontend-api.md`（概览）、`CLAUDE.md`（认证模型：username 登录 + 邀请码闸门 + env 播种 admin + 角色）同步更新。
3. **测试**：校验器/邀请码库/admin 中间件/播种逻辑单测 + handler 集成（注册各拒绝路径、admin 播种幂等、profile、头像、邀请码轮换 403）+ 前端 Vitest（Profile 表单、邀请码面板显隐）+ Playwright E2E（**邀请码注册 → 登录 → 改 profile** 关键路径，全程不碰 AI/邮件）。
4. **CI**：新增测试自动被既有 `ci.yml`（`go test -race`、`npm run test`、`e2e.yml`）覆盖；E2E 的注册流程需在 workflow 里带上 `INVITE_CODE` 环境变量启动服务。**plan 里为"契约/文档/测试/CI"各列显式任务或步骤**。

## 12. 测试要点（全 mock，不真发信、不碰真实 key）
- 单元：username/password/email/nickname 校验表驱动；邀请码 store（get/rotate）；admin 播种（缺省不建、已存在跳过、非法 username fatal 路径可测函数）；`RequireAdmin`。
- 集成：注册成功自动登录；邀请码错/格式错/重复 username/email 各拒绝；username 登录；profile 更新校验；头像上传类型/大小；admin 查看/轮换 + 非 admin 403。
- 契约 E2E：无验证码的新流程（邀请码注册→me→改 profile→admin 查看/轮换）逐个 `ValidateResponse`。

## 13. 安全说明
- 主闸门 = 邀请码（env/admin 掌控），叠加已有每用户积分上限 → 双重防额度滥用。
- admin 凭证由 ops 用 env 掌控，避免"首个注册者夺权"。
- 邀请码仅 admin 可见（403 门禁），轮换即时失效旧码。
- email 本期不验证，作为 deferred 增强（字段/UX/流程已为其预留）。
