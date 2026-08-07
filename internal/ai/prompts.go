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

// PanelDraft is a single storyboard frame proposed by the model. CharacterIDs
// and SceneID reference the ids listed in the AssetContext (0/empty when none).
type PanelDraft struct {
	Caption      string  `json:"caption"`
	CharacterIDs []int64 `json:"characterIds"`
	SceneID      int64   `json:"sceneId"`
	ImagePrompt  string  `json:"imagePrompt"`
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

// conversePrompt builds the system prompt for open-ended storyboard discussion.
func conversePrompt(assets AssetContext) string {
	return tonePrompt + "\n\n" + assetList(assets) +
		"\n请与用户一起讨论、打磨这一章的分镜创意。回答保持简洁、温暖、有画面感。"
}

// storyboardPrompt builds the system prompt instructing the model to emit a
// strict JSON array of exactly n panels indexing the listed asset ids.
func storyboardPrompt(assets AssetContext, n int) string {
	return tonePrompt + "\n\n" + assetList(assets) +
		fmt.Sprintf(
			"\n现在请生成这一章的分镜脚本，共 %d 个画面。"+
				"只输出一个 JSON 数组，不要输出任何解释、前后缀或代码块标记。"+
				"数组必须恰好包含 %d 个对象，每个对象包含以下键：\n"+
				"- caption：中文旁白/台词，简短温暖；\n"+
				"- characterIds：出现的角色 id 数组（引用上面列出的角色 id；没有则为空数组 []）；\n"+
				"- sceneId：场景 id（引用上面列出的场景 id；没有则为 0）；\n"+
				"- imagePrompt：英文绘图提示词，描述该画面，吉卜力/宫崎骏风格。\n"+
				"再次强调：只输出 JSON 数组本身。",
			n, n,
		)
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
