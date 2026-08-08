# oh-my-commic 系统设计与架构

> 本文是 **系统设计与架构的权威入口**：设计目标、架构图、数据模型、核心流程、部署拓扑，以及全部分功能设计文档的索引。
> 提示词原文与细粒度模块/文件地图见 [`ARCHITECTURE-AND-PROMPTS.md`](ARCHITECTURE-AND-PROMPTS.md)；API 契约见 [`openapi.yaml`](openapi.yaml)。

---

## 1. 是什么 / 设计目标

面向小朋友 & 亲子的 **AI 漫画书**：上传家人/宠物照片 → AI 漫画化成**锁定形象** → 对话式拆镜 → 逐格出图 → 拼成可翻页绘本 → 发布到社区。

设计上有四条贯穿始终的原则：

1. **多用户隔离是第一要务**：每个数据访问都能追溯并校验 `user_id`；跨用户/不存在一律 **404**（不泄露存在性）。
2. **不可变**：service 返回新对象，不原地改入参。
3. **小文件、单一职责**：17 个领域模块，模块间只经公开接口交互（构造注入）。
4. **契约驱动**：前后端以 `openapi.yaml` 为单一真相，契约测试对每个响应校验，防漂移。

---

## 2. 系统架构（分层 + 外部依赖）

```mermaid
flowchart TB
    subgraph Client["浏览器"]
        SPA["React SPA<br/>(Vite + TS + Tailwind)"]
    end

    subgraph Server["Go 单进程 (cmd/server)"]
        HTTPX["httpx: chi 路由 + SPA 托管 + /media + 请求日志"]
        MW["auth 中间件<br/>RequireUser / RequireAdmin / OptionalUser"]
        subgraph Handlers["handler 层 (HTTP 边界, 校验输入)"]
            H["auth · book · asset · chapter<br/>panel · story · render · community"]
        end
        subgraph Services["service 层 (业务编排, 不可变)"]
            S["auth · book · asset · comicify<br/>chapter · panel · story · render · community"]
        end
        subgraph Repos["repository 层 (WHERE ... AND user_id=?)"]
            R["各模块 Repo + auth.UserRepo(积分账本 Spend/Refund)"]
        end
        AI["ai.Client<br/>(文本 + 生图 + 错误分类)"]
    end

    subgraph Store["持久化 (具名卷 omc-data)"]
        DB[("SQLite<br/>纯 Go, 幂等迁移")]
        FS[["本地图片目录<br/>/media/{bookId}/"]]
    end

    subgraph Vendors["外部 AI 供应商"]
        QWEN["通义千问 qwen-plus<br/>(DashScope) 文本"]
        SEED["火山方舟 Seedream 4.0<br/>生图 (≤10 参考图)"]
    end

    SPA -->|"/api/v1/* (openapi 契约)"| HTTPX
    SPA -->|"/media/*"| HTTPX
    HTTPX --> MW --> H --> S --> R --> DB
    S --> FS
    S -.出图/拆镜.-> AI
    AI -->|文本| QWEN
    AI -->|生图| SEED
```

**要点**：单进程同时托管 SPA + API + 图片；`auth.UserRepo` 用一个窄接口 `Spend/Refund` 充当积分账本，被 `render`/`comicify` 依赖（接口隔离）；所有 AI 调用集中在 `ai.Client`，上游错误在此分类为 429/504/502，**绝不外泄 body 与 key**。

---

## 3. 数据模型（ER 图）

```mermaid
erDiagram
    USER  ||--o{ BOOK      : owns
    BOOK  ||--o{ CHARACTER : has
    BOOK  ||--o{ SCENE     : has
    BOOK  ||--o{ CHAPTER   : has
    CHAPTER ||--o{ PANEL   : has
    BOOK  ||--o{ BOOK_LIKE : "被点赞"
    BOOK  ||--o{ BOOK_VIEW : "被浏览"
    USER  ||--o{ BOOK_LIKE : "点赞"

    USER {
        int64  id PK
        string username "登录用, 唯一"
        string nickname "社区展示名"
        string email "收集不验证"
        string role "admin | user"
        string avatarUrl
        int    credits "积分, 默认100"
        string passwordHash "不外泄"
    }
    BOOK {
        int64  id PK
        int64  userId FK
        string title
        string coverUrl
        string style
        string summary
        bool   isPublic
        int    likeCount
        int    viewCount
        string publishedAt
    }
    CHARACTER {
        int64  id PK
        int64  bookId FK
        string type "character | pet"
        string name
        string imageUrl "锁定形象"
    }
    SCENE {
        int64  id PK
        int64  bookId FK
        string name
        string imageUrl "锁定形象"
    }
    CHAPTER {
        int64  id PK
        int64  bookId FK
        int    order
        string status "draft→storyboarding→rendering→done"
        bool   isCover
        string summary
        json   conversation "对话历史"
        int    panelCount
    }
    PANEL {
        int64  id PK
        int64  chapterId FK
        int    order
        string content "第1段产出"
        string caption "旁白"
        json   characterIds "出场角色"
        json   charExpressions "各自表情"
        int64  sceneId
        string location
        string event
        string imagePrompt "第2段产出"
        string imageUrl
        string status
    }
    BOOK_LIKE {
        int64  bookId PK "复合主键去重"
        int64  userId PK
    }
    BOOK_VIEW {
        int64  bookId PK "复合主键去重"
        string viewerKey PK "u:{id} | anon:{clientId}"
    }
    SETTINGS {
        string key PK "如 invite_code"
        string value
    }
```

- **一本书 = 一个宇宙**：`Book` 顶层单元，自带 `Character/Scene` 库跨章复用。
- **社区两表**：`book_likes`、`book_views` 均用**复合主键去重**；feed 索引 `idx_books_public_published`。
- **全局邀请码**存 `settings` 表、可轮换。

---

## 4. 核心流程时序（以"逐格出图"为例）

展示隔离校验、积分预扣/退还、参考图锁一致性如何串起来：

```mermaid
sequenceDiagram
    autonumber
    participant FE as 前端 SPA
    participant MW as auth 中间件
    participant RH as render.Handler
    participant RS as render.Service
    participant UR as UserRepo(积分)
    participant AI as ai.Client
    participant SD as Seedream
    participant FS as 本地图片目录

    FE->>MW: POST /api/v1/panels/{id}/render (cookie)
    MW->>MW: 解析 session, 注入 userID
    RH->>RS: Render(userID, panelID)
    RS->>RS: panel→chapter→book→user 链式归属校验
    Note over RS: 越权/不存在 → 404 (不泄露存在性)
    RS->>UR: Spend(userID, cost)
    Note over UR: 余额不足 → 402, 绝不调用付费模型
    RS->>AI: 组装 stylePrefix+结构字段+参考图绑定
    AI->>SD: 生图 (≤10 张锁定形象参考图)
    alt 上游失败/超时
        SD--)AI: 错误
        AI--)RS: 分类 429/504/502
        RS->>UR: Refund(userID, cost)
        RS--)FE: 对应错误码 + 友好提示
    else 成功
        SD--)AI: 图片 URL
        AI->>FS: 下载落地 /media/{bookId}/
        RS--)FE: panel.imageUrl (done)
    end
```

---

## 5. 两段式提示词编排（核心 AI 设计）

把"一句话 → 可出图的画面"拆成**内容 / 结构 / 出图**三个**可干预**阶段——这也是产品"把 agent 从黑盒变玻璃盒"的技术基础：

```mermaid
flowchart LR
    U(["家长/孩子<br/>讲一句话"])
    subgraph Stage1["① 拆镜对话 storyboardChatPrompt"]
        S1["LLM 温暖回应<br/>+ 拆成恰好 N 格 content"]
    end
    subgraph Stage2["② 逐格解析 processPanelPrompt"]
        S2["每格 content →<br/>地点/出场/表情<br/>事件/旁白/imagePrompt"]
    end
    subgraph Stage3["③ 出图 render.buildPrompt"]
        S3["stylePrefix + 结构字段<br/>+ 参考图绑定(角色=图N)"]
    end
    IMG["Seedream → 落地 /media"]
    U --> S1 -->|content 可编辑| S2 -->|每项可微调/重解析| S3 --> IMG
```

拆两段的收益：单次 LLM 负担更小、更不易截断成非法 JSON；每格可**独立重试**；中间态**摊开可编辑**。提示词原文见 [`ARCHITECTURE-AND-PROMPTS.md` §3](ARCHITECTURE-AND-PROMPTS.md)。

---

## 6. 部署拓扑

```mermaid
flowchart LR
    User["用户浏览器"] -->|"http://commic.1501.fun"| Host

    subgraph Host["服务器 (Docker host)"]
        direction TB
        C["容器 oh-my-commic<br/>(sevenoxin/oh-my-commic:latest)<br/>宿主 80 → 容器 8080"]
        V[("具名卷 omc-data:/data<br/>SQLite + 图片")]
        ENV[".env (env_file 运行时注入)<br/>DASHSCOPE_API_KEY / ARK_API_KEY"]
        C --- V
        ENV -.注入.-> C
    end

    subgraph CICD["GitHub Actions"]
        CI["ci.yml: build+vet+race / 前端 test+build"]
        E2E["e2e.yml: Playwright"]
        PUB["docker-publish.yml → Docker Hub"]
    end
    CICD -.push main 触发.-> Reg["Docker Hub<br/>sevenoxin/oh-my-commic"]
    Reg -.docker compose pull.-> C
```

**密钥只在 `.env`（`env_file` 运行时注入），绝不进镜像、绝不入 git**；数据落具名卷；健康探针 `/api/health` + `restart: unless-stopped`。部署与升级步骤见 [README「🐳 Docker 部署」](../README.md#-docker-部署)。

---

## 7. 模块边界与依赖

17 个领域模块，每个只暴露 `NewRepo / NewService / NewHandler`，模块间**只依赖对方 service 接口**，在 `cmd/server/main.go` 用构造注入装配。完整模块表（职责边界 / 对外暴露 / 消费谁的接口）见 [交付说明 DELIVERABLE.md 第二部分 §1.1](DELIVERABLE.md)，细粒度文件地图见 [ARCHITECTURE-AND-PROMPTS.md §2](ARCHITECTURE-AND-PROMPTS.md)。

---

## 8. 设计文档索引（分功能 spec）

系统是**分功能迭代**的，每个功能有独立设计文档（`docs/superpowers/specs/`）与实施计划（`docs/superpowers/plans/`）：

| 功能 | 设计文档 |
|---|---|
| 初版整体设计（骨架） | [`2026-08-07-oh-my-commic-design.md`](superpowers/specs/2026-08-07-oh-my-commic-design.md) |
| 对话式拆镜 | [`2026-08-07-conversational-storyboard-design.md`](superpowers/specs/2026-08-07-conversational-storyboard-design.md) |
| 两段式分镜 | [`2026-08-08-two-stage-storyboard-design.md`](superpowers/specs/2026-08-08-two-stage-storyboard-design.md) |
| 出图阶段改进 | [`2026-08-08-render-stage-improve-design.md`](superpowers/specs/2026-08-08-render-stage-improve-design.md) |
| 书封面 | [`2026-08-08-book-cover-design.md`](superpowers/specs/2026-08-08-book-cover-design.md) |
| 阅读器 | [`2026-08-08-book-reader-design.md`](superpowers/specs/2026-08-08-book-reader-design.md) |
| 用户管理（邀请码/角色/积分） | [`2026-08-08-user-management-design.md`](superpowers/specs/2026-08-08-user-management-design.md) |
| 社区（公开只读 + 点赞/浏览） | [`2026-08-08-community-design.md`](superpowers/specs/2026-08-08-community-design.md) |
| 社区应用壳（左栏 tab + 内容区） | [`2026-08-08-community-shell-design.md`](superpowers/specs/2026-08-08-community-shell-design.md) |

---

## 9. 相关文档

- [`README.md`](../README.md) — 快速开始 / Docker 部署 / 环境变量
- [`DELIVERABLE.md`](DELIVERABLE.md) — 按评审维度的交付说明（五部分）
- [`USAGE.md`](USAGE.md) — 图文使用教程
- [`ARCHITECTURE-AND-PROMPTS.md`](ARCHITECTURE-AND-PROMPTS.md) — 模块/文件地图 + AI 提示词原文
- [`openapi.yaml`](openapi.yaml) — API 契约（单一真相）
- `CLAUDE.md` — 迭代上手指南 + 核心约定 + 踩过的坑
