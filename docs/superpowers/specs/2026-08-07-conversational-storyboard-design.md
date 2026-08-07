# 对话式结构化分镜 设计文档

- **日期**: 2026-08-07
- **目标**: 把"讲故事"环节从"自由聊天 + 另起一步生成 JSON"合并成**一个连续对话**——每轮用户输入，LLM 返回"一句话回应 + 整套结构化分镜"，用户持续对话打磨；每格分镜含**地点、人物(各带表情)、事件**，每格出场资产（角色+场景）**≤3**。

## 1. 交互与数据流
```
用户说一句 → POST /api/chapters/{id}/storyboard-chat {messages:[...]}
           → LLM 一次调用 → { reply, panels:[结构化分镜] }
           → 后端 ReplacePanels 持久化本轮 panels(草稿), 章节置 storyboarding
           → 前端右侧分镜列表实时刷新
用户继续说(带完整历史) → 再来一轮 → 分镜被持续打磨
满意 → 「确认分镜」→ 进入逐格出图(不再重新生成)
```
旧 `/converse` + `/storyboard` 两接口合并为一个 `storyboard-chat`。

## 2. 分镜结构（每格 PanelDraft）
```jsonc
{
  "location": "黄昏的森林餐桌旁",          // 地点(文字)
  "sceneId": 2,                           // 场景资产 id(可选, 0=无; 用于参考图)
  "characters": [                         // 人物+表情
    { "id": 7, "expression": "歪着头、耳朵微抖、好奇" }
  ],
  "event": "棉花糖听到门开的声音，回头张望", // 事件
  "caption": "棉花糖蹲在餐桌边，好奇地回头——门开了。", // 中文旁白(阅读)
  "imagePrompt": "英文绘图提示(含地点/各角色及表情/事件)"
}
```
**≤3 硬约束**: `len(characters) + (sceneId!=0 ? 1 : 0) ≤ 3`；角色优先。与 render 端 `qwen-image-edit-plus` 最多 3 图一致。

## 3. 数据模型（panels 表）
保留 `character_ids / scene_id / caption / image_prompt / order / status`。新增列（幂等 `ALTER TABLE ADD COLUMN`，旧库不重建）：
- `location TEXT NOT NULL DEFAULT ''`
- `event TEXT NOT NULL DEFAULT ''`
- `char_expressions TEXT NOT NULL DEFAULT '{}'` — JSON `{ "角色id": "表情" }`

`character_ids` 仍由 `characters[].id` 派生（供 render 选参考图）。

## 4. LLM Prompt（一次调用，返回 reply+panels）
system:
```
你是温柔的儿童绘本编剧(吉卜力/宫崎骏语气)。
【可用角色】id/名字/简介   【可用场景】id/名字
与用户对话打磨这一章分镜。每次只输出一个 JSON:
{ "reply": "给用户的一句温暖回应",
  "panels": [ { "location","sceneId","characters":[{"id","expression"}],"event","caption","imagePrompt" } ] }
规则:
- 每格必须有 地点 / 人物(含表情) / 事件;
- 每格出场角色数 + (有场景?1:0) ≤ 3;
- characters[].id 和 sceneId 只能引用上面列出的 id;
- imagePrompt 用英文, 含地点、每个角色及其表情、事件, 吉卜力风格;
- 只输出这个 JSON, 无多余文字/代码块标记。
```
鲁棒解析: 截首个 `{` 到末个 `}` → `json.Unmarshal`。**校验**: 过滤非法 character/scene id; 每格截到 ≤3(角色优先); 表情只保留挂在合法角色上的。

## 5. Render 端 prompt
由 `吉卜力风格前缀 + location + 每个出场角色(名字+表情) + event` 拼成; 参考图仍是出场角色/场景锁定图(≤3, 已实现)。renderer 读取新字段(location/event/char_expressions)来构造更精确的 prompt。

## 6. API
- `POST /api/chapters/{id}/storyboard-chat` (protected): body `{messages:[{role,content}]}` → `{reply:string, panels:Panel[]}`; 内部: 校验章节归属 → 载资产 → LLM → 解析+校验 → ReplacePanels → SetStatus storyboarding → 返回 reply+持久化后的 panels。
- 移除/弃用 `/converse` 与 `/storyboard`(前端改用新接口)。
- `PUT /api/panels/{id}` 扩展: 允许编辑 location/event/characters(表情)/caption。

## 7. 前端（Stage ① 重做 ChatStoryboard）
左: 对话气泡(用户/AI) + 输入框; 右: 结构化分镜卡列表, 每格分区 📍地点 / 🧑人物(表情) / ⚡事件, 可分别编辑; 每轮 AI 回复后刷新; 底部「确认分镜 →」进 Stage ② 出图。加载态用 LoadingClouds。

## 8. 隔离/安全/错误
- 章节归属校验不变(story.Service 经 chapter.GetChapter)。
- LLM/上游错误 → 502 通用消息, 不泄露 key。
- 解析失败(非 JSON) → 502 友好提示, 不落库半成品。

## 9. 测试
- ai: `{reply,panels}` 鲁棒解析(含前后噪音/代码块); 每格 ≤3 校验; 非法 id 过滤; 表情绑定合法角色。
- story service: 每轮持久化 panels + 章节状态; 跨用户 → 404。
- render: 用 location/expressions/event 构造 prompt(既有多图参考不变)。
- 前端: 构建通过; 对话→分镜实时更新; 单格字段编辑。
