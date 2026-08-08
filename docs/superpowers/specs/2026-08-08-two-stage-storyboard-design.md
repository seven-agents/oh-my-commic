# 两段式分镜（对话拆镜 + 单格解析）+ 对话/分镜持久化 设计文档

- **日期**: 2026-08-08
- **背景**: 现状——讲故事的对话历史和分镜数**没有持久化**（`ChatStoryboard` 的 messages 是纯前端 state），刷新/重进章节聊天框就空了，用户**无法接着聊来微调**；单格只能改文本字段，**不能让 AI 独立重新解析某一格**。
- **目标**: 把 LLM 交互**拆成两段**，并持久化对话，支持"整体改（对话）/单格改（直接改+重新解析）"。

## 1. 核心模型
```
【第1段·讲故事对话 StoryboardChat】
  用户输入(+对话历史 + 当前各格 content 作上下文) → LLM 补充剧情 + 拆成 N 格【基本内容 content】
  → 输出 {reply, summary, panels:[{content}]}（只做拆镜+基础内容，不含结构化字段）
  → 持久化：对话(conversation) + 分镜数(panel_count) + 各格 content
  ▸ 想改整体分镜 → 在对话框继续聊（对话历史恢复、可接着微调）

【第2段·逐格解析 ProcessPanel（手动触发）】
  对某一格(或"全部解析") → LLM 从该格 content + 书里角色/场景
  → 输出该格 {location, characters:[{id,expression}], sceneId, event, caption, imagePrompt}
  → 存并展示（含出图提示词 imagePrompt，可编辑）
  ▸ 想改单独一格 → 直接改这格 content / 结构字段 / imagePrompt → 「重新解析这格」或直接出图

【出图】基于结构字段 + 参考图 渲染（不变）
```

## 2. 数据模型
### 2.1 Panel（加 content）
- 新增 `content TEXT NOT NULL DEFAULT ''`：第1段产出的**基本分镜内容**（"这一格发生了什么"，可直接编辑）。
- 原有 `location / character_ids / char_expressions / scene_id / event / caption / image_prompt`：第2段解析结果（可展示、可编辑，含 imagePrompt 可编辑）。
- `models.Panel` 加 `Content string \`json:"content"\``。

### 2.2 Chapter（加 conversation + panel_count）
- 新增 `conversation TEXT NOT NULL DEFAULT '[]'`：JSON 数组 `[{"role","content"}...]`，第1段对话历史。
- 新增 `panel_count INTEGER NOT NULL DEFAULT 6`。
- `models.Chapter` 加 `Conversation []ConversationMsg \`json:"conversation"\``（`ConversationMsg{Role,Content string}`）+ `PanelCount int \`json:"panelCount"\``。

### 2.3 迁移
- 幂等 `ALTER TABLE ADD COLUMN`（panels.content / chapters.conversation / chapters.panel_count），旧库不重建。老 panel content 为空、结构字段与图片保留、chapter 无历史对话——不受影响。

## 3. 第1段：StoryboardChat 改造
- **Prompt**：只要求把故事拆成 N 格，每格给一段**中文基本内容 content**（发生了什么、谁在场、大概场景，一两句），外加 `reply`(温暖回应) 与 `summary`(整章概述)。**不再要求结构化字段**（那是第2段）。
- **当前分镜上下文**：调用前后端加载该章**当前各格 content**（用户可能已改），拼进 prompt 作"当前分镜现状"，让 LLM 从现状**微调**而非从零重写。
- **输出 JSON**：`{ "reply", "summary", "panels":[{"content"}...] }`；鲁棒解析（首个`{`到末个`}`）；缺字段容错。分镜数 N 走已有"恰好 N 格"硬指令 + panelCount。
- **持久化 + 保留既有成果（关键）**：把 LLM 返回的 N 个 content 与**现有 panels 按序做 content 合并**：
  - 位置 i 的新 content == 现有第 i 格 content → **完整保留该格**（结构字段 + imageUrl + status 不动）；
  - content 变了 → 更新 content，**清空该格结构字段与 image、status 置 pending**（需重新解析/出图）；
  - 多出的格 → 新增(仅 content)；少的 → 删除。
  - 用事务 ReplacePanels 落库（合并后的完整列表）。
  - 同时写入 chapter.conversation（= 请求 messages + 本轮 assistant reply）、panel_count、summary（非空才覆盖）。章节状态置 storyboarding（已存在则跳过）。

## 4. 第2段：ProcessPanel（新增）
- `POST /api/panels/{id}/process`（protected）：归属校验（panel→chapter→book）→ 载书角色/场景 → AI 从 `panel.content` + 资产清单 → 输出该格 `{location, characters:[{id,expression}], sceneId, event, caption, imagePrompt}`（英文 imagePrompt 含地点/角色表情/事件，吉卜力风）→ 服务端**净化**（非法 id 过滤、≤10 参考、表情绑合法角色）→ `panel.Update` 写入结构字段 → 返回更新后的 panel。
- 单格触发；"全部解析"由前端对每个未解析/已改动的格循环调用。
- ai 层新增 `ProcessPanel(ctx, c, content, assets) (PanelDraftV2, error)`（复用 PanelDraftV2 结构与净化）。

## 5. 编辑接口
- `PUT /api/panels/{id}` 扩展可编辑字段：新增 `content`；保留 `caption/characterIds/sceneId/imagePrompt/location/event/charExpressions`（imagePrompt 可编辑）。
- 改整体 → 对话框(第1段)；改单格 → 直接 PUT 改 content/结构/imagePrompt，或「重新解析这格」(第2段)。

## 6. 前端
- **进入 Chapter 编辑器**：`GET /api/chapters/:id` 带回 `conversation` + `panelCount` → `ChatStoryboard` 用它初始化 messages 与分镜数（**聊天框恢复历史**）；`GET panels` 带回各格 content + 结构字段。
- **Stage① 对话**：左对话(恢复历史)，右侧分镜卡显示**每格 content**（内联可编辑，改完 PUT）。
- **Stage② 逐格卡**：每格显示 content + 「解析这格」；解析后展开 地点/人物(表情)/场景/事件/旁白/**出图提示词**（均内联可编辑，imagePrompt 可编辑）+「重新解析这格」；顶部「全部解析」。未解析的格给出提示。
- **出图**：沿用逐格渲染（用结构字段 + 参考图）。
- 复用 ui/ 组件、LoadingClouds、错误处理；ref 防重入。

## 7. 隔离/错误/测试
- 全部走现有归属校验；AI/解析错误 → 502 通用（不泄露 key）。
- 测试：
  - 迁移幂等加列；chapter conversation/panel_count 往返；panel content 往返。
  - StoryboardChat 解析 `{reply,summary,panels:[{content}]}`；**content 合并**：未变格保留结构+image、变化格清空、增删正确；对话+panel_count 持久化。
  - ProcessPanel 从 content 产出结构字段并净化（非法 id 过滤、≤10）；跨用户 404。
  - 前端 build 通过；对话历史恢复；单格 content 编辑 + 重新解析 + imagePrompt 编辑。
