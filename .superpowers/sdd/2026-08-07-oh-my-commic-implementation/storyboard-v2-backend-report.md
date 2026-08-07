# 对话式结构化分镜（后端）实现报告

日期: 2026-08-07 · 分支: `feat/conversational-storyboard`

## 目标
把"讲故事"从"自由聊天 + 另起一步生成 JSON"合并成一次连续对话。每轮用户输入，
LLM 返回 `{reply, panels}`，每格分镜含 地点(location) / 人物(含表情) / 事件(event)，
每格出场资产(角色+场景) ≤ 3。每轮持久化 panels。

## 变更概览（按层）

### 1. 数据模型 `internal/models/models.go`
`Panel` 新增三字段（保留 `CharacterIDs`，仍供 render 选参考图）：
- `Location string json:"location"`
- `Event string json:"event"`
- `CharExpressions map[int64]string json:"charExpressions"` — 角色 id → 表情

### 2. 迁移 `internal/db/migrate.go`
- 新库：`panels` CREATE TABLE 内联三列
  `location TEXT NOT NULL DEFAULT ''`、`event TEXT NOT NULL DEFAULT ''`、
  `char_expressions TEXT NOT NULL DEFAULT '{}'`。
- 旧库：新增 `alterStatements` 切片，`Migrate` 在 CREATE 之后逐条执行
  `ALTER TABLE panels ADD COLUMN ...`；`isDuplicateColumn`（匹配
  "duplicate column name"）忽略"列已存在"错误 → 幂等，新库/旧库/重复运行都不报错。
- 测试新增 `TestPanelsStructuredColumns`（新库列存在，PRAGMA table_info 检查）和
  `TestMigrateAddsColumnsToLegacyDB`（丢弃并用旧结构重建 panels → Migrate 后三列补齐，
  再次 Migrate 幂等）。

### 3. panel 仓储/服务 `internal/panel/repo.go`
- `panelColumns` 增加 `location, event, char_expressions`。
- `ReplaceForChapter` INSERT、`ListByChapter`/`Get` 的 `scanPanel` 均含三列。
- `Update` 扩展可编辑集：`caption, character_ids, scene_id, image_prompt,
  location, event, char_expressions`（`Service.UpdatePanel` 透传，归属校验不变）。
- 新增表情 map JSON 助手（对齐既有 characterIds 助手）：
  `marshalExpressions`（nil→`{}`）、`unmarshalExpressions`（空/空白→空 map；
  JSON 键为字符串，用 `strconv.ParseInt` 还原 int64 键）、`normalizeExpressions`。

### 4. ai 结构化对话 `internal/ai/storyboardchat.go`(新) + `prompts.go`
- 新类型：`CharacterRef{ID,Expression}`、`PanelDraftV2{Location,SceneID,Characters,
  Event,Caption,ImagePrompt}`、`StoryboardResult{Reply,Panels}`。
- `StoryboardChat(ctx,c,history,assets)`：system(=`storyboardChatPrompt`) + history →
  `c.Chat`；`parseStoryboardResult` 截首个 `{` 到末个 `}`（容忍散文/```json``` 代码块）→
  `json.Unmarshal` 到 `StoryboardResult`；`sanitizePanels` 逐格校验。
- **校验/清洗**（`sanitizePanel`，常量 `maxPanelRefs=3`）：丢弃非法角色 id；
  非法/外部 sceneId 置 0；强制 ≤3（角色优先：角色≥3 则丢场景并截到 3，否则角色+场景>3 时丢场景）；
  表情只随保留的角色存续。
- `storyboardChatPrompt(assets)`：tone + assetList + 只输出一个 `{reply,panels:[...]}`
  JSON 对象；规则含 每格必须有地点/人物(带表情)/事件、≤3、只引用列出的 id、
  imagePrompt 英文含地点/各角色及表情/事件。
- 删除旧 `Converse`/`GenStoryboard`/`PanelDraft`/`conversePrompt`/`storyboardPrompt`
  及其测试，保留 `assetList`/`tonePrompt`/`firstNonEmpty`/`AssetContext`。
- 新测试 `storyboardchat_test.go`：散文包裹 `{reply,panels}` 解析（reply/location/event/
  Characters[0].Expression），非法角色 id + 第 4 个引用被过滤(≤3、非法丢弃)；
  3 角色+场景 → 丢场景；```json``` 代码块变体；无对象 → error；malformed → error。

### 5. story 统一 `internal/story/{service.go,handler.go}`
- `service.go`：以 `StoryboardChat(userID,chapterID,history) (reply, panels, err)`
  取代 `Converse`+`GenerateStoryboard`。校验归属+载资产 → `ai.StoryboardChat` →
  `draftsToPanels`（`PanelDraftV2`→`models.Panel`：CharacterIDs=[chars 的 id]、
  CharExpressions={id:expression}、Location/Event/SceneID/Caption/ImagePrompt、
  Status="pending"）→ `ReplacePanels` → 首轮才 `SetStatus` storyboarding（已在则跳过，
  避免自环 ErrInvalidStatus）→ 返回 reply+已存 panels。`chapter.ErrNotFound`→`ErrNotFound`。
  ai 层已清洗，`draftsToPanels` 只做扁平化。
- `handler.go`：单一路由 `POST /api/chapters/{id}/storyboard-chat`，body `{messages:[...]}`，
  返回 `{reply, panels}`。移除 converse/storyboard 路由与处理器及 panelCount。
  AI/解析错误 → 502 通用消息(不泄露 key)；ErrNotFound→404；decode→400。
  测试全部改写为新契约。

### 6. render prompt `internal/render/service.go`
`buildPrompt` 拼入 `p.Location`（"地点：…"）、`p.Event`（"事件：…"），
`characterSummary(c, p.CharExpressions[c.ID])` 把每个匹配角色的表情作为"表情…"并入括号描述。
参考图选择（≤3、角色优先）不变。

## API 变更
- 移除：`POST /api/chapters/{id}/converse`、`POST /api/chapters/{id}/storyboard`。
- 新增：`POST /api/chapters/{id}/storyboard-chat`
  - 请求：`{"messages":[{"role","content"}]}`
  - 响应 200：`{"reply": string, "panels": Panel[]}`（Panel 含 location/event/charExpressions）
  - 404 归属/未知章节；400 请求体错误；502 AI/解析失败（通用消息，不泄露 key）
- `PUT /api/panels/{id}` 现可持久化 location/event/charExpressions（经 repo.Update 扩展）。

## 构建/测试结果
- `go build ./...` 干净
- `go vet ./...` 干净
- `gofmt -l` 干净
- `go test -race ./...` 全部 ok（ai/panel/story/db/render/httpx 等）

## 关注点 / 后续
- `≤3` 双重保障：ai 层清洗 + render 层 `modelMaxRefs=3` 钳制，防御到位。
- 归属/隔离不变（story.Service 经 chapter.GetChapter；panel.Service 经 ChapterOwner）。
- `char_expressions` JSON 键为字符串（Go map[int64]string 序列化即字符串键），读回用
  ParseInt 还原；表情键与 CharacterIDs 可能不完全一致（若模型给了重复/多余表情），
  但只对匹配到的角色使用，render 侧安全。
- 前端（Stage ① ChatStoryboard 重做）属另一批工作，本次仅后端。
