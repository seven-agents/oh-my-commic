# oh-my-commic 模块地图 & AI 提示词汇总

> 面向后续迭代的参考文档。**提示词全部为源码原文**，改提示词时按下方"位置"定位即可。
> 最后更新：2026-08-08。

---

## 1. 技术栈 / 供应商

| 层 | 选型 |
|---|---|
| 后端 | Go 单体，chi 路由，分层 handler → service → repository，SQLite（modernc 纯 Go）+ 本地图片目录 |
| 前端 | React + Vite + TS + Tailwind（SPA，由 Go 服务托管 web/dist）|
| **文本 LLM** | 通义千问 DashScope `qwen-plus`，compatible-mode `https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions`，`max_tokens=8192` |
| **图像生成** | 火山方舟 Seedream `doubao-seedream-4-0-250828`，`https://ark.cn-beijing.volces.com/api/v3/images/generations`，Bearer；参数 `size=2048x2048, sequential_image_generation=disabled, watermark=false`；**最多 10 张参考图**（base64） |
| 密钥 | `.env`（`DASHSCOPE_API_KEY` / `ARK_API_KEY`），env_file 注入，不入镜像 |

详见 `internal/config/config.go`（默认值/env 名）与记忆 `oh-my-commic-ai-models-verified`。

---

## 2. 模块地图

### 后端 `internal/`
| 包 | 职责 |
|---|---|
| `models` | 所有数据结构（User/Book/Character/Scene/Chapter/Panel/ConversationMsg）|
| `config` | 环境变量加载（模型 id、base url、上限、密钥）|
| `db` | SQLite 连接 + 迁移（幂等 ALTER）；FK 级联 + `foreign_keys(1)` DSN |
| `auth` | 注册/登录/session（持久化到 SQLite）+ 隔离中间件 `RequireUser` |
| `book` | Book CRUD + `SetCover`（渲染封面章时同步 cover_url）|
| `asset` | 角色/宠物/场景 CRUD + 上传归属校验 |
| `comicify` | **资产漫画化**：上传图 → Seedream 图生图 → 锁定形象图（见 §3.4）|
| `chapter` | 章节 CRUD + 状态机 + 封面章(is_cover) + conversation/panel_count/summary 持久化 |
| `panel` | 分镜 CRUD（content + 结构字段 + 图片），批量替换/排序 |
| `community` | **社区跨表只读**：公开 feed 列表 + 公开阅读详情（严格 `is_public`，非公开/不存在 404，作者仅 nickname/avatar）+ 点赞/独立访客浏览计数（`book_likes`/`book_views`）|
| `ai` | **千问文本客户端 + 两段式分镜提示词 + Seedream 生图客户端**（见 §3）|
| `render` | **单格出图编排**：拼 prompt + 参考图 → Seedream → 下载落地 → 回写 panel（见 §3.3）|
| `story` | 编排层：storyboard-chat(第1段) + process-panel(第2段) |
| `imageutil` | 参考图缩放（≤768px，喂模型前压缩）|
| `storage` | 本地图片存储（按 book_id 目录，路径安全）|
| `httpx` | 路由组装（Deps）+ SPA 托管 + 请求日志 |

### 前端 `web/src/`
| 组件/页面 | 职责 |
|---|---|
| `pages/Login` | 登录/注册 |
| `pages/Home` | 公开首页（两入口卡：社区 / 我的漫画）|
| `pages/Community` | 社区 feed（公开，无需登录）|
| `pages/CommunityReader` | 公开阅读器（公开阅读详情，复用 `BookReaderView`）|
| `pages/Bookshelf` | 书架（`/my`，受保护；阅读/编辑/删除按钮，创建卡在最后；每卡公开/私密开关）|
| `pages/BookWorkspace` | 演员表 + 章节列表(封面卡 + 第一章…) |
| `pages/AssetEditor` | 角色/场景 上传 + 设定（保存时触发 comicify）|
| `pages/ChapterEditor` | 分镜编辑器（分阶段：讲故事→解析出图→拼成书）|
| `pages/BookReader` | 整本书翻页阅读（封面大图 + 每章一页 + AI 概述）|
| `components/ChatStoryboard` | 第1段对话（恢复历史 + 分镜数选择）|
| `components/StoryboardPanelCard` | 单格可编辑卡（content 输入 + 结构输出 + imagePrompt + 重新解析）|
| `components/PanelGrid`/`PanelCard` | 出图阶段（**并发全部生成**；卡默认收起编辑、按需展开）|
| `components/ComicCompose`/`ComicPage` | 拼版 |
| `components/chapter/panelStage` | `canRenderPanel`（未解析禁止出图）|
| `components/CommunityCard` | 社区 feed 单卡（封面 + 作者 nickname/avatar + 点赞/浏览数）|
| `components/reader/BookReaderView` | 从 `BookReader` 抽出的**展示层**，owner 私有阅读与社区公开阅读共用 |

---

## 3. AI 提示词汇总（**改提示词看这里**）

分镜与 LLM 的交互分**两段**：第1段对话拆镜（只出基本内容 content），第2段逐格解析（出结构化字段 + 出图提示词）。出图时 render 再把结构字段拼成最终 Seedream prompt。

### 3.0 共用语气 tonePrompt · 位置 `internal/ai/prompts.go`
```
你是一位温柔的儿童绘本编剧，擅长吉卜力工作室、宫崎骏风格的画面想象。你的语气温暖、充满童趣与善意，画面明亮柔和，适合小朋友阅读。
```
`assetList()` 把书里角色/场景带 id 列进 prompt（供模型索引）：`【可用角色】（引用时使用 id）：- id=.. 名字=.. 简介=..`。

### 3.1 第1段 · 拆镜对话 `storyboardChatPrompt` · 位置 `internal/ai/prompts.go`
tone + assetList + 分镜数指令 + (当前分镜现状) + 下述指令。**只出 content，不出结构化字段**。输出：
```json
{ "reply": "给用户的一句温暖回应",
  "summary": "这一章故事的一段温暖、简短(2~4句)的中文概述",
  "panels": [ { "content": "这一格的中文基本内容（什么场景、谁在场、发生了什么，一两句）" } ] }
```
- **分镜数** `panelCountInstruction`：`【分镜数量-重要】你必须把这个故事拆成【恰好 N 格】…绝对不要只输出 1 格。`（N≤0 时用"4~8 格"）。
- **微调上下文** `currentContentsBlock`：有现有分镜时插入 `【当前分镜现状（请在此基础上微调，不要推倒重来）】：第1格：… 第2格：…`，让模型从现状改而非重写。
- 解析容错：`flexID`（id 可为数字/字符串/名字，非法降级为 0 被过滤）；鲁棒截取首个`{`到末个`}`。

### 3.2 第2段 · 逐格解析 `processPanelPrompt` · 位置 `internal/ai/prompts.go`
输入=某格的中文 content（user 消息）+ assetList。输出该格结构化：
```json
{ "location": "地点（中文）", "sceneId": 场景id或0,
  "characters": [ { "id": 角色id, "expression": "表情/神态" } ],
  "event": "事件（中文）", "caption": "中文旁白/台词，简短温暖",
  "imagePrompt": "中文绘图提示词" }
```
规则：只解析这一格；必须有 地点/人物(带表情)/事件；出场角色+场景 **≤10**；id 只能引用列表、不编造；**imagePrompt 用中文**，含地点/各角色表情/事件、吉卜力风。服务端 `sanitizePanel` 再次过滤非法 id 并 ≤10。

### 3.3 出图 · render 拼装最终 prompt · 位置 `internal/render/service.go` `buildPrompt`
按顺序拼：
```
stylePrefix + [画面：caption。] + [地点：location。] + 角色X（性别/年龄/性格/表情）。… + [场景Y（描述）。] + [事件：event。] + [补充：imagePrompt。] + 参考图绑定
```
- **风格前缀 + 不要文字**（`stylePrefix`）：
  `吉卜力/宫崎骏风格：手绘水彩、暖色调、柔和光影、圆润造型、亲子友好绘本风。画面中不要出现任何文字、字母、数字或水印。画面内容铺满整幅，四周不要任何边框、画框、描边或色条。`
- **参考图绑定**（多角色一致性关键）：把出场角色/场景的**锁定图**按序 base64 传入（上限 `RENDER_MAX_REFS=10`，Seedream 支持最多 10 张），并在 prompt 里写明：
  `本次提供了 N 张参考图，请严格按参考图还原对应对象的样貌…参考图1=角色棉花糖；参考图2=角色芝麻糊；…`
- `characterSummary`/`sceneSummary` 生成上面的角色/场景中文描述串。

### 3.4 资产漫画化 comicify · 位置 `internal/comicify/prompts.go`
上传角色/场景图 → Seedream 图生图 → 锁定形象图。
- 共用 `styleBase`：`把参考图重绘成宫崎骏吉卜力风格的手绘水彩绘本插画：暖色调、柔和光影、圆润线条、亲子友好、干净简洁的背景。画面铺满整幅，四周不要任何边框、画框、描边或色条。`
- 角色 `characterPrompt`：+`保留…关键外貌特征与神态，画成完整的单个角色形象，居中构图…` + 名字/性别/年龄/性格/描述。
- 场景 `scenePrompt`：+`画成一张绘本背景/场景图，只保留环境与氛围，画面中不要出现任何人物或角色。` + 场景名/描述。

---

## 4. 端到端数据流

```
建书 → 传角色/场景图 → comicify 漫画化成锁定形象图(§3.4)
新建章节 → 第1段讲故事对话(§3.1) → 各格 content + 对话/分镜数持久化
        ▸ 改整体：继续对话（带当前 content 微调）
逐格解析(§3.2 手动) → 每格 结构字段 + 中文 imagePrompt
        ▸ 改单格：改 content/结构/imagePrompt → 重新解析 或 直接出图
逐格出图(§3.3) → stylePrefix+字段+参考图绑定 → Seedream(≤10图) → 下载落地 /media/{bookId}/
拼成书 → 阅读器翻页(封面大图 + 每章一页 + AI 概述)
```

## 5. 迭代提示词的checklist
1. 改文本 → `internal/ai/prompts.go`（拆镜/解析）、`internal/render/service.go`（出图拼装 + stylePrefix）、`internal/comicify/prompts.go`（漫画化）。
2. 改上限/模型 → `internal/config/config.go`（`RENDER_MAX_REFS`、`SEEDREAM_MODEL`、`QWEN_TEXT_MODEL`）。
3. 结构字段变动 → 同步 `PanelDraftV2`/`sanitizePanel`(ai)、`models.Panel`、migrate、前端 `types.ts`。
4. 改完 `go test ./...` + 真机 smoke（storyboard-chat / process / render 各跑一次看输出）。
