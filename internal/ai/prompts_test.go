package ai

import (
	"strconv"
	"strings"
	"testing"
)

// TestStoryboardChatPromptTargetsCount verifies that a positive panelCount is
// woven into the prompt as an explicit target and that the multi-panel + user
// override instructions are always present.
func TestStoryboardChatPromptTargetsCount(t *testing.T) {
	const want = 4
	p := storyboardChatPrompt(AssetContext{}, want)

	if !strings.Contains(p, strconv.Itoa(want)) {
		t.Fatalf("prompt 应包含目标格数 %d: %q", want, p)
	}
	if !strings.Contains(p, "拆成多格分镜") {
		t.Fatalf("prompt 应要求拆成多格分镜: %q", p)
	}
	if !strings.Contains(p, "以用户的要求为准") {
		t.Fatalf("prompt 应说明以用户对话中的格数为准: %q", p)
	}
	if strings.Contains(p, "4~8") {
		t.Fatalf("指定格数时不应出现默认区间: %q", p)
	}
}

// TestStoryboardChatPromptDefaultRange verifies that a non-positive panelCount
// falls back to the default 4~8 range wording.
func TestStoryboardChatPromptDefaultRange(t *testing.T) {
	for _, count := range []int{0, -1} {
		p := storyboardChatPrompt(AssetContext{}, count)
		if !strings.Contains(p, "4~8 格分镜") {
			t.Fatalf("panelCount=%d 应使用默认区间 4~8: %q", count, p)
		}
		if !strings.Contains(p, "拆成多格分镜") {
			t.Fatalf("panelCount=%d prompt 应要求拆成多格分镜: %q", count, p)
		}
		if !strings.Contains(p, "以用户的要求为准") {
			t.Fatalf("panelCount=%d prompt 应说明以用户要求为准: %q", count, p)
		}
	}
}
