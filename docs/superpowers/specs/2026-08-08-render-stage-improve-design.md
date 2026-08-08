# 出图阶段改进（并发生成 + 不要文字 + 单格可编辑再解析）设计文档

- **日期**: 2026-08-08
- **背景/问题**:
  1. **「全部生成」看起来卡住**：`PanelGrid.renderAll` 是**串行 await**，每格出图 30-40s，后面的格要等前一格完才开始，用户点了只看到第 1 格在"生成中"，其余无反应。
  2. Seedream 出图偶尔在画面里加**乱码文字/字母/水印**。
  3. 出图阶段的卡片只能出图、不能就地**改内容/改提示词/重新解析**——用户想微调只能退回上一步。

## 1. 「全部生成」改为并发（修 bug）
- `renderAll` 从"串行 for-await"改为**并发**：对所有【已解析(`canRenderPanel`)且非 done】的格用 `Promise.all(renderable.map(renderOne))` 同时发起。
- 每格进入时**立即**函数式 `applyPanel(id,{status:'rendering'})`（所有格同时显示"生成中"），各自 await 自己的 `POST /api/panels/{id}/render`，成功 `applyPanel(id, done)`、失败标 `failed`（互不影响）。
- 保持函数式更新（避免 stale 覆盖）；`bulk` 期间禁用按钮 + ref 防重入。
- 未解析的格仍跳过（有第3点的「重新解析」可就地处理）。
- 格数少，先全并发；若上游压力大后续可限并发（本设计先不限）。

## 2. 出图提示词加"不要文字"约束（后端）
- `internal/render/service.go` 的 render prompt 末尾固定追加一句：**「画面中不要出现任何文字、字母、数字或水印。」**（放在 `stylePrefix` 或 buildPrompt 结尾常量）。
- 对所有出图统一生效；纯 prompt 文本改动（Seedream 无独立 negative 参数）。
- 加/改一个后端小测：`buildPrompt` 结果包含该约束串。

## 3. 出图阶段卡片：可编辑输入/输出 + 重新解析（前端）
出图阶段的每格卡（`PanelCard`/`PanelGrid`）增强为"可编辑 + 重解析 + 出图"：
- **解析输入**：`content`（这格基本内容，内联可编辑 → `PUT /api/panels/{id}` 带 content）。
- **解析输出（结构字段，均内联可编辑 → PUT）**：📍地点 `location`、⚡事件 `event`、📝旁白 `caption`、🧑出场角色+表情（`characterIds`+`charExpressions`，可删角色 chip）、🎨**出图提示词 `imagePrompt`**（Textarea 可改）。
- **🔁 重新解析这格**：`POST /api/panels/{id}/process`（从当前 content 重新推导输出，覆盖结构字段），loading + ref 防重入。
- **🎨 生成这张图**：`POST /api/panels/{id}/render`，`canRenderPanel` 门控（未解析禁用并提示）。
- **出图预览**：done 显示图片，可"重新生成"。
- 复用 stage② `StoryboardPanelCard` 的编辑 UI（内联可编辑字段 + 角色 chip + imagePrompt），把它扩展/复用到出图阶段并加上「重新解析 / 生成图 / 图片预览」；或让出图阶段直接复用增强后的同一卡片组件，避免两套重复。

## 4. 兼容/隔离/错误
- 全部走现有归属校验；AI/渲染错误 → 现有 502/failed 态，友好提示，不泄露 key。
- 未解析格出图仍被门控（避免无内容图）。

## 5. 测试
- 后端：`buildPrompt` 含"不要出现任何文字…"约束；现有 render 测试仍绿。
- 前端：`npm run build` 通过；「全部生成」并发（所有可渲染格同时进 rendering）；出图阶段单格 改 content→重新解析→输出更新→出图；imagePrompt 就地编辑生效。
