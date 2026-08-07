package comicify

import (
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// styleBase is the shared Ghibli / Miyazaki storybook art-direction prefix used
// when redrawing an uploaded reference into the locked stylized asset image.
const styleBase = "把参考图重绘成宫崎骏吉卜力风格的手绘水彩绘本插画：暖色调、柔和光影、圆润线条、亲子友好、干净简洁的背景。"

// characterPrompt builds the Chinese instruction to redraw the person/creature
// in the reference image as a full Ghibli storybook character, weaving in the
// character's metadata as guidance while preserving its key features.
func characterPrompt(c models.Character) string {
	var b strings.Builder
	b.WriteString(styleBase)
	b.WriteString("保留画面主体（人物或生物）的关键外貌特征与神态，画成完整的单个角色形象，居中构图，全身或半身清晰可见。")

	parts := make([]string, 0, 5)
	if c.Name != "" {
		parts = append(parts, "名字"+c.Name)
	}
	if c.Gender != "" {
		parts = append(parts, "性别"+c.Gender)
	}
	if c.Age != "" {
		parts = append(parts, "年龄"+c.Age)
	}
	if c.Personality != "" {
		parts = append(parts, "性格"+c.Personality)
	}
	if c.Description != "" {
		parts = append(parts, "描述"+c.Description)
	}
	if len(parts) > 0 {
		b.WriteString("角色设定作为参考：")
		b.WriteString(strings.Join(parts, "、"))
		b.WriteString("。")
	}
	return b.String()
}

// scenePrompt builds the Chinese instruction to redraw the reference image as a
// Ghibli storybook background/scene with no characters present.
func scenePrompt(sc models.Scene) string {
	var b strings.Builder
	b.WriteString(styleBase)
	b.WriteString("画成一张绘本背景/场景图，只保留环境与氛围，画面中不要出现任何人物或角色。")

	parts := make([]string, 0, 2)
	if sc.Name != "" {
		parts = append(parts, "场景名"+sc.Name)
	}
	if sc.Description != "" {
		parts = append(parts, "描述"+sc.Description)
	}
	if len(parts) > 0 {
		b.WriteString("场景设定作为参考：")
		b.WriteString(strings.Join(parts, "、"))
		b.WriteString("。")
	}
	return b.String()
}
