# oh-my-commic 🎨📖

面向小朋友 & 亲子的 **AI 漫画书**制作 Web 应用。
用户建一本书，创建自己的角色与场景，用一句话 + 和 AI 聊分镜，逐格生图，最后拼成一本属于自己的漫画书——默认**宫崎骏 / 吉卜力**风格。

> 48 小时面试作品。设计文档见 [`docs/superpowers/specs/2026-08-07-oh-my-commic-design.md`](docs/superpowers/specs/2026-08-07-oh-my-commic-design.md)。

---

## ✨ 核心特性

- **多用户隔离** —— 每个用户的书 / 角色 / 章节严格隔离。
- **一本书一个宇宙** —— Book 顶层单元，自带角色与场景，跨章节复用。
- **AI 索引资产** —— AI 读故事，自动决定每个分镜出场哪些角色、用哪个场景。
- **参考图锁一致性** —— 上传的角色/场景图作参考图，同一角色跨分镜形象一致。
- **对话式分镜** —— 输入一段介绍，和 AI 来回聊，聊出一组分镜脚本。
- **逐格生图 → 拼版成书** —— 一张张生成，再拼版成漫画页。

## 🧩 组织结构

```
User
 └─ Book  (一本漫画书 = 顶层单元)
     ├─ Character  (角色/宠物, 跨章节复用)
     ├─ Scene      (场景, 跨章节复用)
     └─ Chapter    (故事/章节)
         └─ Panel   (分镜: pic1, pic2 …)
```

## 🔄 制作流程

```
建书 → 建资产(上传图+设定) → 新建章节
     → 和 AI 对话生成分镜脚本 → AI 索引角色/场景
     → 逐格生图(参考图锁一致) → 拼版成页 → 阅读
```

## 🖥️ 页面（6）

| 页面 | 职责 |
|---|---|
| P1 登录注册 | 唯一入口 |
| P2 书架主页 | 我的书封面墙 |
| P3 Book 工作台 | 管角色/场景 + 章节 |
| P4 资产编辑 | 上传图 + 填设定 |
| P5 Chapter 编辑器 | 对话分镜 → 逐格生图 → 拼版（核心） |
| P6 漫画阅读 | 翻页看成品 |

## 🏗️ 技术栈

| 层 | 选型 |
|---|---|
| 前端 | React + Vite + TypeScript + Tailwind (SPA) |
| 后端 | Go + chi，分层 handler → service → repository |
| 存储 | SQLite + 本地图片目录 |
| AI·对话逻辑 | 通义千问 DashScope 文本 `qwen-plus`（讲故事拆镜 + 逐格解析）→ `DASHSCOPE_API_KEY` |
| AI·出图逻辑 | 火山方舟 Seedream `doubao-seedream-4-0-250828`（逐格出图 + 资产漫画化，最多 10 张参考图）→ `ARK_API_KEY` |

## 🚀 快速开始

先配置环境变量（两种模式都需要）：

```bash
cp .env.example .env
#   在 .env 填入两个 Key（缺任一后端启动即 fatal）：
#     DASHSCOPE_API_KEY —— 通义千问，对话/文本逻辑（拆镜、解析）
#     ARK_API_KEY       —— 火山方舟，出图逻辑（Seedream 逐格出图、资产漫画化）
```

### 一键 demo（单服务，推荐面试演示）

前端先构建成静态文件，Go 服务同时托管 **SPA + API + 图片**，一个端口搞定：

```bash
cd web && npm install && npm run build && cd ..
go run ./cmd/server
```

打开 http://localhost:8080 —— 前端页面、`/api/*`、`/media/*` 全部由这一个进程提供。
（服务启动时检测到 `web/dist/index.html` 便自动开启 SPA 托管；找不到则退回 API-only 模式。）

### 开发模式（前后端分离，热更新）

两个终端，前端走 Vite 热更新、通过代理把 `/api`、`/media` 打到 :8080：

```bash
# 终端 1：后端
go run ./cmd/server

# 终端 2：前端（Vite dev server）
cd web && npm install && npm run dev
```

打开 http://localhost:5173 开发（Vite 已配置 `/api`、`/media` 代理到 `http://localhost:8080`）。

## 🔐 环境变量

| 变量 | 说明 | 默认值 |
|---|---|---|
| `DASHSCOPE_API_KEY` | 通义千问 DashScope Key —— **对话/文本逻辑**（拆镜、解析）（**必填，只放 .env**） | — |
| `ARK_API_KEY` | 火山方舟 Ark Key —— **出图逻辑**（Seedream 逐格出图、资产漫画化）（**必填，只放 .env**） | — |
| `PORT` | 服务监听端口 | `8080` |
| `DB_PATH` | SQLite 文件路径 | `oh-my-commic.db` |
| `DATA_DIR` | 上传/生成图片的本地存储目录 | `data` |
| `WEB_DIST` | 前端构建产物目录（存在 `index.html` 时开启 SPA 托管） | `web/dist` |
| `QWEN_TEXT_MODEL` | 对话/文本模型 id | `qwen-plus` |
| `QWEN_TEXT_BASE_URL` | 文本接口 base url | DashScope 兼容模式 |
| `SEEDREAM_MODEL` | 出图模型 id | `doubao-seedream-4-0-250828` |
| `SEEDREAM_BASE_URL` | 出图接口 base url | 火山方舟 Ark v3 |
| `RENDER_MAX_REFS` | 每格出图最多参考图数（≤10） | `10` |

> ⚠️ 切勿把 API key 提交到 git。`.env`、`*.db`、`web/dist/`、`web/node_modules/` 均已在 `.gitignore` 中排除。

## 📦 项目结构（规划）

```
oh-my-commic/
├── cmd/server/          # Go 入口
├── internal/
│   ├── auth/            # M1 认证 + 隔离中间件
│   ├── book/            # M2
│   ├── asset/           # M3 角色/宠物/场景
│   ├── chapter/         # M4
│   ├── panel/           # M5 分镜
│   ├── ai/              # M6 AI 网关：千问文本编排 + 火山方舟 Seedream 生图
│   └── storage/         # M7 图片落地
├── web/                 # React 前端
├── docs/superpowers/specs/
└── .env                 # 本地密钥(gitignored)
```

## 🎬 演示脚本

3 分钟面试演示走位见 [`docs/DEMO.md`](docs/DEMO.md)。

## 📅 状态

前后端打通 ✅ · 单服务一键 demo 可运行 ✅
