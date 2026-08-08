# 整本书翻页阅读器 + 章节 AI 概述 设计文档

- **日期**: 2026-08-08
- **目标**: 目前用户只能在编辑器里一页页看分镜，缺少"阅读整本书"的体验。新增：① 书架每本书卡片加「📖 阅读」「✏️ 编辑」两个按钮；② 整本书的翻页阅读器——**一章一页**，每页是该章拼好的漫画版式 + 章节标题 + **AI 润色的故事概述**；③ 章节概述由 `storyboard-chat` 那次调用顺带产出并保存（不额外多调 AI）。

## 1. 数据流
```
讲故事(storyboard-chat) 每轮 → AI 返回 { reply, summary, panels }
  → summary(该章故事概述, 中文2-4句) 存进 chapters.summary(每轮更新)
阅读: 书架点「阅读」→ /books/:id/read
  → 加载 书 + 全部章节(按 order, 各带 summary) + 各章 panels
  → 封面页 + 每章一页(拼版漫画 + 标题 + summary) → 翻页浏览
```

## 2. 数据模型（chapters 表）
- 新增列 `summary TEXT NOT NULL DEFAULT ''`（幂等 `ALTER TABLE ADD COLUMN`，旧库不重建）。
- `models.Chapter` 加 `Summary string \`json:"summary"\``。
- chapter repo：`ListByBook`/`Get`/`Create` 的 SELECT/INSERT 带上 `summary`；新增 `SetSummary(chapterID, summary)`（或在现有更新路径写入）。

## 3. AI 概述（并进 storyboard-chat）
- `ai.StoryboardResult` 从 `{Reply, Panels}` 扩为 `{Reply, Summary, Panels}`（`summary string json:"summary"`）。
- `storyboardChatPrompt` 追加：让模型在同一个 JSON 里输出 `"summary"` —— **基于用户讲的故事，润色成一段温暖、简短(2-4句)的中文故事概述**，用于书页展示。
- 鲁棒解析不变（首个`{`到末个`}`）；summary 缺失时容忍为空串。
- `story.Service.StoryboardChat`：拿到 `res.Summary` 后 `chapters.SetSummary(userID, chapterID, summary)`（归属校验复用 GetChapter）。summary 为空则不覆盖已有(可选:直接覆盖也可，简单起见覆盖)。

## 4. 前端

### 4.1 书架卡片（`BookCard`）
- 封面下方加两个按钮：**「📖 阅读」**→ `/books/:id/read`，**「✏️ 编辑」**→ `/books/:id`。
- 右上角删除 ✕ 保留。按钮 `stopPropagation`，样式复用 ui/ 组件与暖色。

### 4.2 阅读器（新页面 `BookReader`, 路由 `/books/:id/read`）
- 加载：`GET /api/books/:id`、`GET /api/books/:id/chapters`、逐章 `GET /api/chapters/:cid/panels`。
- **页序列**：
  - 第 0 页 = **封面页**：书名 + 封面(BookCover) + 书简介(book.summary)。
  - 之后每个"**有已渲染分镜**的章节" = 一页：
    - 章节标题(页眉)
    - **拼版漫画**：复用 `ComicCompose`/`ComicPage`，把该章 `status=done`/有 imageUrl 的分镜按网格排版。
    - **故事概述**：`chapter.summary`（在漫画下方或侧边，温暖排版）。
- **翻页交互**：左右大箭头「‹ ›」+ 键盘 ← →，页码「第 X / N 页」，轻微翻页过渡；顶部返回书架 + 「去编辑」链接。
- **空状态**：整本书没有任何已渲染分镜 → 友好 EmptyState（"这本书还没画好的漫画，先去编辑画几页吧～"）；单个没图的章节跳过不成页。
- 复用现有 ui/ 组件、AppHeader、LoadingClouds、错误处理(errorMessage + 全局 401)。

## 5. 隔离/错误
- 全部走现有归属校验（books/chapters/panels 均 user 隔离）。summary 写入经 GetChapter 归属校验。
- AI/解析错误 → 502 通用（不泄露 key）；阅读器读取失败 → 友好提示。

## 6. 测试
- 后端：`StoryboardChat` 解析 `{reply,summary,panels}`（summary 有/无都容忍）；service 每轮存 summary；chapter repo summary 往返；migrate 幂等加列；跨用户 404。
- 前端：`npm run build` 通过；书架两按钮跳转正确；阅读器翻页、封面页、空状态、章节 summary 展示。
