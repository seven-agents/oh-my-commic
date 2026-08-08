# 封面章 + 章节序号修复 设计文档

- **日期**: 2026-08-08
- **目标**:
  1. **修复章节序号 off-by-one**：后端 `order` 是 1-based（第一章 order=1），前端 `ChapterList` 显示 `ch.order + 1` → 显示成 2、3…。改为正确显示「第一章、第二章…」。
  2. **新增封面制作**：封面 = **特殊的第一章**（`is_cover`，order=0，锁定 1 格），复用完整 讲故事→分镜(1格)→出图 流程；渲染后自动同步为 `book.coverUrl`。展示顺序：**封面 → 第一章 → 第二章 …**

## 1. 数据模型（chapters 表）
- 新增列 `is_cover INTEGER NOT NULL DEFAULT 0`（幂等 `ALTER TABLE ADD COLUMN`，旧库不重建）。
- `models.Chapter` 加 `IsCover bool \`json:"isCover"\``。
- `chapterColumns`/`scanChapter`/Create/List/Get 带上 `is_cover`。

## 2. 后端行为
### 2.1 封面章创建（单例，order=0）
- `POST /api/books/:id/cover-chapter`：若该书已有 `is_cover=1` 的章 → 直接返回它；否则创建 `is_cover=1, order=0, title="封面", status="draft"`。归属校验复用。
- 普通章 Create 不变（`COALESCE(MAX("order"),0)+1`；封面 order=0，第一个真章 = 1）。
- `Repo`：加 `CreateCover(bookID)`（order=0, is_cover=1）与 `FindCover(bookID)`；`Service.EnsureCover(userID, bookID)` 归属门控，返回封面章（有则取、无则建）。

### 2.2 封面同步 coverUrl
- `render.RenderPanel` 成功写回 panel 图后：若该 panel 所属章 `IsCover`，则把 `book.coverUrl` 设为该图 URL。
- `book.Repo` 加 `SetCover(userID, bookID, url) error`（`UPDATE books SET cover_url=?, updated_at=? WHERE id=? AND user_id=?`）。render.Service 需要一个 book cover 设置口：注入一个 `CoverSetter` 接口 `SetCover(userID,bookID,url)error`（由 *book.Repo 满足）。ownership 已在 render 链路确认（panel→chapter→book 属该 user），传入 userID。
- 失败 log-and-continue（不因封面同步失败而让渲染失败）。

## 3. AI / 渲染
- 不改 storyboard/render 逻辑：封面章走同样的 storyboard-chat（前端传 `panelCount=1`）+ 逐格出图；封面也能索引书里的角色/场景做参考图（≤10）。

## 4. 前端
### 4.1 类型
- `Chapter` 加 `isCover: boolean`。

### 4.2 Book 工作台章节区（`ChapterList`）
- 最前面一个 **封面卡**：
  - 若存在封面章 → 显示「封面」+ 封面缩略(该章已渲染的分镜图/或 book.coverUrl)，点击进封面编辑（`/chapters/:coverChapterId`）。
  - 若不存在 → 「＋ 制作封面」卡；点击调 `POST /api/books/:id/cover-chapter` 拿到封面章 → 跳转其编辑器。
- 普通章节（`isCover=false`）：**序号修复** —— 显示「第 {N} 章」，N = 该章在非封面章中的名次（等于 `order`，1-based）。用中文数字（第一章…第十章，超过用「第N章」阿拉伯数字兜底）。去掉原来的 `+1`。
- 排序：封面卡永远在最前；其余按 order 升序。

### 4.3 ChapterEditor（封面模式）
- 若章 `isCover`：
  - 顶部标题/文案改为"制作封面"，对话引导词偏封面（"用一句话描述这本书的封面吧～"）。
  - **锁定 panelCount=1**（隐藏分镜数选择器，storyboard-chat 传 1）。
  - 其余（对话→出图→拼版/保存）流程复用；"拼成书"这步对封面可简化为"保存封面"（把该格图设为封面，已由渲染同步 coverUrl 完成；点保存回工作台即可）。

### 4.4 阅读器（`BookReader`）
- 封面页(page 0) 继续显示 `book.coverUrl`（现在是 AI 封面）。
- **每章一页时排除 `isCover` 的章**（封面不再单独成一页）。
- 章节页标题若显示序号，同样用「第一章…」修复。

## 5. 隔离/错误
- 所有新接口/写入走现有归属校验（books/chapters/panels/user 隔离）。封面 coverUrl 同步经 userID 校验的 SetCover。
- AI/渲染错误 → 现有 502/失败态；封面同步失败 log-and-continue。

## 6. 测试
- 后端：is_cover 迁移幂等；`EnsureCover` 单例(重复调用返回同一封面章)、order=0、归属；渲染封面章 panel → book.coverUrl 被更新（非封面章不动 coverUrl）；跨用户 404。
- 前端：build 通过；封面卡（有/无封面章两态）；普通章序号显示「第一章、第二章」；封面编辑锁 1 格；阅读器排除封面章、封面页用 coverUrl。
