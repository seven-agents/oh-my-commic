package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// maxPanelRefs is the hard per-panel reference cap: characters + (scene?1:0).
// It mirrors the render layer's image-model limit (Seedream 4.0 accepts up to
// 10 reference images), so a panel can never demand more references than the
// image model can consume. Enforced server-side as defense-in-depth.
const maxPanelRefs = 10

// flexID is an int64 that tolerates the model emitting an id as EITHER a JSON
// number (1) or a JSON string ("1"). LLMs are inconsistent about this and a
// rigid int64 field makes the whole storyboard parse fail (→ 502) whenever the
// model quotes an id. Empty string / null decode to 0.
type flexID int64

func (f *flexID) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexID(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		// A non-integer string (e.g. the model quoting a character NAME like
		// "小狐狸" instead of an id) is treated as "no valid id" → 0, which the
		// sanitizer then drops. This must NOT fail the whole storyboard parse.
		if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			*f = flexID(v)
		} else {
			*f = 0
		}
		return nil
	}
	// null or any other shape → treat as "no id".
	*f = 0
	return nil
}

// CharacterRef is a character present in a panel together with the expression /
// demeanor the model wants rendered for that character in that frame.
type CharacterRef struct {
	ID         flexID `json:"id"`
	Expression string `json:"expression"`
}

// PanelDraftV2 is one structured storyboard frame proposed by the model: a
// setting (Location + optional SceneID), the present characters with their
// expressions, the event, a Chinese caption, and an English image prompt.
type PanelDraftV2 struct {
	Location    string         `json:"location"`
	SceneID     flexID         `json:"sceneId"`
	Characters  []CharacterRef `json:"characters"`
	Event       string         `json:"event"`
	Caption     string         `json:"caption"`
	ImagePrompt string         `json:"imagePrompt"`
}

// StoryboardResult is one conversational turn's output: a warm one-line reply to
// the user, a polished Chinese story overview (Summary) for the book reader, plus
// the full structured storyboard for the chapter. Summary is optional — a model
// that omits it decodes to the empty string and must not fail the parse.
type StoryboardResult struct {
	Reply   string         `json:"reply"`
	Summary string         `json:"summary"`
	Panels  []PanelDraftV2 `json:"panels"`
}

// StoryboardChat runs one turn of the unified conversational storyboard flow. It
// prepends the structured system prompt to the accumulated history, calls the
// chat model, robustly extracts the JSON object embedded in the reply (tolerating
// surrounding prose and code fences), and sanitizes every panel against the
// book's real assets before returning. panelCount is a soft target threaded into
// the system prompt (0 = let the prompt use its default range); the user may
// still override it in conversation.
func StoryboardChat(ctx context.Context, c *Client, history []Msg, assets AssetContext, panelCount int) (StoryboardResult, error) {
	messages := append([]Msg{{Role: "system", Content: storyboardChatPrompt(assets, panelCount)}}, history...)

	content, err := c.Chat(ctx, messages)
	if err != nil {
		return StoryboardResult{}, err
	}

	res, err := parseStoryboardResult(content)
	if err != nil {
		return StoryboardResult{}, err
	}

	res.Panels = sanitizePanels(res.Panels, assets)
	return res, nil
}

// parseStoryboardResult extracts the JSON object embedded in content (from the
// first '{' to the last '}') and unmarshals it into a StoryboardResult. It
// returns an error when no JSON object is present or the fragment is malformed.
func parseStoryboardResult(content string) (StoryboardResult, error) {
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start < 0 || end < 0 || end < start {
		return StoryboardResult{}, fmt.Errorf("ai: no JSON object found in storyboard response")
	}

	fragment := content[start : end+1]

	var res StoryboardResult
	if err := json.Unmarshal([]byte(fragment), &res); err != nil {
		return StoryboardResult{}, fmt.Errorf("ai: parse storyboard JSON: %w", err)
	}
	return res, nil
}

// sanitizePanels validates and trims each panel against the book's real assets:
//   - character refs whose id is not a real character are dropped;
//   - a sceneId that is not a real scene is reset to 0;
//   - the ≤10 total-references rule is enforced, characters first (if there are
//     already 10+ characters the scene is dropped; characters are capped at 10).
//
// Expressions survive only for the characters that are kept.
func sanitizePanels(panels []PanelDraftV2, assets AssetContext) []PanelDraftV2 {
	validChars := make(map[int64]struct{}, len(assets.Characters))
	for _, ch := range assets.Characters {
		validChars[ch.ID] = struct{}{}
	}
	validScenes := make(map[int64]struct{}, len(assets.Scenes))
	for _, sc := range assets.Scenes {
		validScenes[sc.ID] = struct{}{}
	}

	out := make([]PanelDraftV2, 0, len(panels))
	for _, p := range panels {
		out = append(out, sanitizePanel(p, validChars, validScenes))
	}
	return out
}

// sanitizePanel applies the asset-validity and ≤10-reference rules to one panel.
func sanitizePanel(p PanelDraftV2, validChars, validScenes map[int64]struct{}) PanelDraftV2 {
	// Keep only characters that reference a real character id.
	chars := make([]CharacterRef, 0, len(p.Characters))
	for _, cr := range p.Characters {
		if _, ok := validChars[int64(cr.ID)]; ok {
			chars = append(chars, cr)
		}
	}

	// Reset a foreign/hallucinated scene id to "no scene".
	scene := p.SceneID
	if _, ok := validScenes[int64(scene)]; !ok {
		scene = 0
	}

	// Enforce ≤10 total references, characters first. Cap characters at 10; if
	// characters already fill the budget, drop the scene.
	if len(chars) >= maxPanelRefs {
		chars = chars[:maxPanelRefs]
		scene = 0
	} else if scene != 0 && len(chars)+1 > maxPanelRefs {
		scene = 0
	}

	p.Characters = chars
	p.SceneID = scene
	return p
}
