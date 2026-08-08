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
	p := storyboardChatPrompt(AssetContext{}, want, nil)

	if !strings.Contains(p, strconv.Itoa(want)) {
		t.Fatalf("prompt 应包含目标格数 %d: %q", want, p)
	}
	if !strings.Contains(p, "恰好") {
		t.Fatalf("prompt 应硬性要求恰好的格数: %q", p)
	}
	if !strings.Contains(p, "绝对不要只输出 1 格") {
		t.Fatalf("prompt 应明确禁止只输出 1 格: %q", p)
	}
	if !strings.Contains(p, "用户") {
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
		p := storyboardChatPrompt(AssetContext{}, count, nil)
		if !strings.Contains(p, "4~8") {
			t.Fatalf("panelCount=%d 应使用默认区间 4~8: %q", count, p)
		}
		if !strings.Contains(p, "绝对不要只输出 1 格") {
			t.Fatalf("panelCount=%d prompt 应明确禁止只输出 1 格: %q", count, p)
		}
		if !strings.Contains(p, "用户") {
			t.Fatalf("panelCount=%d prompt 应说明以用户要求为准: %q", count, p)
		}
	}
}

// TestStoryboardChatPromptContentOnly verifies the stage-1 prompt asks ONLY for
// content-only panels (a "content" field) and does NOT request the structured
// decomposition fields, which belong to stage 2.
func TestStoryboardChatPromptContentOnly(t *testing.T) {
	p := storyboardChatPrompt(AssetContext{}, 0, nil)
	if !strings.Contains(p, `"content"`) {
		t.Fatalf("stage-1 prompt 应要求 content 字段: %q", p)
	}
	if !strings.Contains(p, "只做拆镜") {
		t.Fatalf("stage-1 prompt 应说明只做拆镜: %q", p)
	}
	// The structured fields must NOT be requested in stage 1.
	if strings.Contains(p, "imagePrompt") || strings.Contains(p, `"location"`) {
		t.Fatalf("stage-1 prompt 不应包含结构化字段: %q", p)
	}
}

// TestStoryboardChatPromptCurrentContents verifies that existing per-frame
// contents are woven in as a "current storyboard" refinement block, and that an
// empty list omits the block entirely.
func TestStoryboardChatPromptCurrentContents(t *testing.T) {
	p := storyboardChatPrompt(AssetContext{}, 0, []string{"棉花糖回头张望", "小狐狸走进门"})
	if !strings.Contains(p, "当前分镜现状") {
		t.Fatalf("有现状时应包含现状区块: %q", p)
	}
	if !strings.Contains(p, "棉花糖回头张望") || !strings.Contains(p, "小狐狸走进门") {
		t.Fatalf("现状区块应列出各格 content: %q", p)
	}

	empty := storyboardChatPrompt(AssetContext{}, 0, []string{"", "  "})
	if strings.Contains(empty, "当前分镜现状") {
		t.Fatalf("全空 content 不应出现现状区块: %q", empty)
	}
}

// TestProcessPanelPromptRequestsStructuredFields verifies the stage-2 prompt asks
// for the structured decomposition (location, characters, event, imagePrompt) of
// a single frame.
func TestProcessPanelPromptRequestsStructuredFields(t *testing.T) {
	p := processPanelPrompt(AssetContext{})
	for _, want := range []string{`"location"`, `"sceneId"`, `"characters"`, `"event"`, `"caption"`, `"imagePrompt"`} {
		if !strings.Contains(p, want) {
			t.Fatalf("stage-2 prompt 应包含字段 %s: %q", want, p)
		}
	}
	if !strings.Contains(p, "不要输出多格") {
		t.Fatalf("stage-2 prompt 应限制只解析一格: %q", p)
	}
}
