package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Converse runs one turn of open-ended storyboard discussion. It prepends a
// system prompt carrying the book's asset list to the accumulated history and
// returns the assistant reply. Multi-turn callers pass the growing history.
func Converse(ctx context.Context, c *Client, history []Msg, assets AssetContext) (string, error) {
	messages := append([]Msg{{Role: "system", Content: conversePrompt(assets)}}, history...)
	return c.Chat(ctx, messages)
}

// GenStoryboard asks the model to emit a storyboard of n panels as a strict JSON
// array, then robustly parses it. It tolerates surrounding prose and code fences
// by extracting the substring from the first '[' to the last ']'.
func GenStoryboard(ctx context.Context, c *Client, history []Msg, assets AssetContext, n int) ([]PanelDraft, error) {
	messages := make([]Msg, 0, len(history)+2)
	messages = append(messages, Msg{Role: "system", Content: storyboardPrompt(assets, n)})
	messages = append(messages, history...)
	messages = append(messages, Msg{Role: "user", Content: fmt.Sprintf("请现在输出这 %d 个画面的 JSON 数组。", n)})

	content, err := c.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	return parseStoryboard(content)
}

// parseStoryboard extracts and unmarshals the JSON array embedded in content.
func parseStoryboard(content string) ([]PanelDraft, error) {
	start := strings.IndexByte(content, '[')
	end := strings.LastIndexByte(content, ']')
	if start < 0 || end < 0 || end < start {
		return nil, fmt.Errorf("ai: no JSON array found in storyboard response")
	}

	fragment := content[start : end+1]

	var drafts []PanelDraft
	if err := json.Unmarshal([]byte(fragment), &drafts); err != nil {
		return nil, fmt.Errorf("ai: parse storyboard JSON: %w", err)
	}

	return drafts, nil
}
