package ai

import (
	"fmt"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// AssetContext carries the book's cast and settings so the model can index them
// by their numeric ids when composing a storyboard.
type AssetContext struct {
	Characters []models.Character
	Scenes     []models.Scene
}

// tonePrompt establishes the warm Ghibli/Miyazaki children's-comic voice shared
// by every request.
const tonePrompt = "你是一位温柔的儿童绘本编剧，擅长吉卜力工作室、宫崎骏风格的画面想象。" +
	"你的语气温暖、充满童趣与善意，画面明亮柔和，适合小朋友阅读。"

// assetList renders the available characters and scenes with their ids so the
// model can reference (index) them precisely. It returns a Chinese description
// noting when a category is empty.
func assetList(assets AssetContext) string {
	var b strings.Builder

	b.WriteString("【可用角色】（引用时使用 id）：\n")
	if len(assets.Characters) == 0 {
		b.WriteString("（暂无角色）\n")
	} else {
		for _, c := range assets.Characters {
			desc := firstNonEmpty(c.Description, c.Personality)
			b.WriteString(fmt.Sprintf("- id=%d 名字=%s 简介=%s\n", c.ID, c.Name, desc))
		}
	}

	b.WriteString("【可用场景】（引用时使用 id）：\n")
	if len(assets.Scenes) == 0 {
		b.WriteString("（暂无场景）\n")
	} else {
		for _, s := range assets.Scenes {
			b.WriteString(fmt.Sprintf("- id=%d 名字=%s\n", s.ID, s.Name))
		}
	}

	return b.String()
}

// storyboardChatPrompt builds the system prompt for the stage-1 conversational
// storyboard turn. The model must reply once with a JSON OBJECT carrying a warm
// one-line reply, a Chinese chapter summary, and a content-only frame list: each
// panel is just a 1-2 sentence Chinese description of that frame (what scene, who
// is present, what happens) — NOT the structured decomposition, which is done
// later in stage 2. When currentContents is non-empty it is listed as the
// current storyboard so the model refines in place rather than rewriting.
func storyboardChatPrompt(assets AssetContext, panelCount int, currentContents []string) string {
	return tonePrompt + "\n\n" + assetList(assets) +
		"\n" + panelCountInstruction(panelCount) +
		currentContentsBlock(currentContents) +
		"\n请与用户对话，一起打磨这一章的分镜。这一步**只做拆镜**：把故事拆成 N 格，每格给一段**中文基本内容 content**——" +
		"一两句话说明这一格【什么场景、谁在场、发生了什么】。**不要**输出地点、人物 id、表情、事件、旁白、绘图提示词等结构化字段（那些留到后续单独解析）。\n" +
		"每次只输出一个 JSON 对象，不要输出任何解释、前后缀或代码块标记：\n" +
		"{\n" +
		"  \"reply\": \"给用户的一句温暖回应\",\n" +
		"  \"summary\": \"这一章故事的一段温暖、简短(2~4句)的中文概述\",\n" +
		"  \"panels\": [\n" +
		"    { \"content\": \"这一格的中文基本内容（什么场景、谁在场、发生了什么，一两句）\" }\n" +
		"  ]\n" +
		"}\n" +
		"规则：\n" +
		"- \"summary\" 是基于用户所讲故事润色出的**一段温暖、简短(2~4句)的中文故事概述**，用于这本书的书页展示（不是旁白，是整章故事的概括）；\n" +
		"- 每一格 \"content\" 只写基本画面内容，用中文，一两句话；\n" +
		"- 不要在 content 里写 id 或英文绘图提示词；\n" +
		"- 只输出这个 JSON 对象本身。"
}

// currentContentsBlock renders the chapter's existing per-frame contents so the
// model refines them in place instead of rewriting from scratch. It returns an
// empty string when there are no current frames.
func currentContentsBlock(currentContents []string) string {
	nonEmpty := false
	for _, c := range currentContents {
		if strings.TrimSpace(c) != "" {
			nonEmpty = true
			break
		}
	}
	if !nonEmpty {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n【当前分镜现状（请在此基础上微调，不要推倒重来）】：\n")
	for i, c := range currentContents {
		b.WriteString(fmt.Sprintf("第%d格：%s\n", i+1, c))
	}
	return b.String()
}

// processPanelPrompt builds the system prompt for the stage-2 per-frame
// decomposition. Given ONE frame's Chinese content (sent as the user message)
// plus the book's asset list, the model must output ONLY a JSON object with the
// structured storyboard fields for that frame. Asset ids constrain what may be
// referenced, and the ≤10-refs rule is stated so the model self-limits (the
// server re-validates regardless).
func processPanelPrompt(assets AssetContext) string {
	return tonePrompt + "\n\n" + assetList(assets) +
		"\n用户会给你**一格分镜的中文基本内容**。请把它解析成这一格的结构化分镜。" +
		"每次只输出一个 JSON 对象，不要输出任何解释、前后缀或代码块标记：\n" +
		"{\n" +
		"  \"location\": \"地点（中文，例如：黄昏的森林餐桌旁）\",\n" +
		"  \"sceneId\": 场景 id（引用上面列出的场景 id；没有则为 0）,\n" +
		"  \"characters\": [ { \"id\": 角色 id, \"expression\": \"该角色的表情/神态\" } ],\n" +
		"  \"event\": \"事件（中文，画面里正在发生什么）\",\n" +
		"  \"caption\": \"中文旁白/台词，简短温暖\",\n" +
		"  \"imagePrompt\": \"英文绘图提示词\"\n" +
		"}\n" +
		"规则：\n" +
		"- 只解析用户给的这一格，不要输出多格；\n" +
		"- 这一格必须有 地点、人物(每个都带表情)、事件；\n" +
		"- 每格出场角色数 + (有场景则 1，否则 0) ≤ 10；角色优先；\n" +
		"- characters[].id 和 sceneId 只能引用上面列出的 id，不要编造；\n" +
		"- imagePrompt 用英文，需包含地点、每个出场角色及其表情、事件，吉卜力/宫崎骏风格；\n" +
		"- 只输出这个 JSON 对象本身。"
}

// panelCountInstruction tells the model to break the chapter into MULTIPLE
// distinct frames. When panelCount > 0 it asks for approximately that many
// panels; otherwise it asks for a sensible default range. Either way, an
// explicit count stated by the user in the conversation takes precedence.
func panelCountInstruction(panelCount int) string {
	if panelCount > 0 {
		return fmt.Sprintf(
			"【分镜数量-重要】你必须把这个故事拆成【恰好 %d 格】分镜，panels 数组里必须【正好包含 %d 个对象】，不能多也不能少。"+
				"即使故事很短，也要展开成 %d 个连续的时刻（例如：开头铺垫→发展→高潮→结尾），每一格是不同的画面。"+
				"绝对不要只输出 1 格。如果用户在对话里另外指定了格数，则以用户说的为准。",
			panelCount, panelCount, panelCount,
		)
	}
	return "【分镜数量】请把这个故事拆成 4~8 格连续分镜，每一格是不同的时刻/情节，" +
		"要展开成多格，绝对不要只输出 1 格。如果用户在对话里指定了格数，以用户说的为准。"
}

// firstNonEmpty returns the first non-empty string among the arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
