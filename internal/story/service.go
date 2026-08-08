// Package story orchestrates AI-driven storyboard conversation and generation:
// it verifies chapter ownership, loads the book's asset context, drives the ai
// gateway, and persists the resulting panels.
package story

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/seven-agents/oh-my-commic/internal/ai"
	"github.com/seven-agents/oh-my-commic/internal/asset"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/models"
	"github.com/seven-agents/oh-my-commic/internal/panel"
)

// ErrNotFound signals that the chapter does not exist or is not owned by the
// requesting user. Handlers map it to 404 so existence is never disclosed.
var ErrNotFound = errors.New("not found")

// pendingStatus is the initial status assigned to freshly generated panels.
const pendingStatus = "pending"

// storyboardingStatus is the chapter status set once a storyboard is generated.
const storyboardingStatus = "storyboarding"

// Service ties the ai gateway to the asset/chapter/panel services.
type Service struct {
	ai       *ai.Client
	assets   *asset.Service
	chapters *chapter.Service
	panels   *panel.Service
}

// NewService returns a Service wired with its collaborators.
func NewService(client *ai.Client, assets *asset.Service, chapters *chapter.Service, panels *panel.Service) *Service {
	return &Service{ai: client, assets: assets, chapters: chapters, panels: panels}
}

// StoryboardChat runs one turn of the stage-1 conversational storyboard flow: it
// verifies chapter ownership, loads the book's assets and the chapter's current
// per-frame contents, asks the model for a content-only {reply, summary, panels}
// result, MERGES the returned contents with the existing panels (preserving the
// structured fields and image of frames whose content is unchanged), persists the
// merged panels via ReplacePanels, persists the conversation + panelCount +
// summary, advances the chapter to storyboarding, and returns the model's reply
// plus the stored panels. panelCount is a soft target for how many frames to
// produce (0 lets the prompt pick a default range). Cross-user or unknown
// chapters yield ErrNotFound; an AI or parse failure is returned as a
// non-ErrNotFound error so nothing half-baked is stored.
func (s *Service) StoryboardChat(userID, chapterID int64, history []models.ConversationMsg, panelCount int) (string, []models.Panel, error) {
	ch, assets, err := s.loadAssets(userID, chapterID)
	if err != nil {
		return "", nil, err
	}

	// Load the chapter's current panels so we can (a) feed their contents to the
	// model as the "current storyboard" and (b) merge the model's new contents
	// back onto them, preserving structured fields and images where unchanged.
	existing, err := s.panels.ListPanels(userID, chapterID)
	if err != nil {
		return "", nil, mapOwnershipErr(err)
	}
	currentContents := make([]string, 0, len(existing))
	for _, p := range existing {
		currentContents = append(currentContents, p.Content)
	}

	res, err := ai.StoryboardChat(context.Background(), s.ai, toAIMsgs(history), assets, panelCount, currentContents)
	if err != nil {
		return "", nil, fmt.Errorf("story: storyboard chat: %w", err)
	}

	merged := mergePanels(chapterID, existing, res.Panels)

	stored, err := s.panels.ReplacePanels(userID, chapterID, merged)
	if err != nil {
		return "", nil, mapOwnershipErr(err)
	}

	// Only advance the state machine on the first turn. On later turns the chapter
	// is already "storyboarding"; there is no self-transition, so calling SetStatus
	// would fail with ErrInvalidStatus even though the panels were replaced
	// successfully. Skipping the no-op keeps the state machine untouched.
	if ch.Status != storyboardingStatus {
		if _, err := s.chapters.SetStatus(userID, chapterID, storyboardingStatus); err != nil {
			return "", nil, mapOwnershipErr(err)
		}
	}

	// Persist the conversation (request history + this turn's assistant reply) and
	// the chosen panel count so the chat editor can restore state next session.
	// Best-effort: the chapter is already known to be owned, so a hiccup here must
	// not discard the successfully stored panels — log and continue.
	nextConv := append(append([]models.ConversationMsg{}, history...), models.ConversationMsg{Role: "assistant", Content: res.Reply})
	if _, err := s.chapters.SaveConversation(userID, chapterID, nextConv, panelCount); err != nil {
		log.Printf("story: persist conversation for chapter %d failed: %v", chapterID, err)
	}

	// Persist the AI-polished story overview for the book reader. Only overwrite
	// when the model produced one, so a turn that omits it never wipes an existing
	// summary. Best-effort as above.
	if res.Summary != "" {
		if _, err := s.chapters.SetSummary(userID, chapterID, res.Summary); err != nil {
			log.Printf("story: persist summary for chapter %d failed: %v", chapterID, err)
		}
	}

	return res.Reply, stored, nil
}

// ProcessPanel runs the stage-2 per-frame decomposition for panelID: it verifies
// ownership, loads the owning chapter's book assets, asks the model to decompose
// the panel's Content into structured fields, and persists them (preserving the
// panel's Content and ImageURL). Cross-user or unknown panels yield ErrNotFound;
// an AI or parse failure is returned as a non-ErrNotFound error.
func (s *Service) ProcessPanel(userID, panelID int64) (models.Panel, error) {
	p, err := s.panels.GetPanel(userID, panelID)
	if err != nil {
		return models.Panel{}, mapOwnershipErr(err)
	}

	ch, err := s.chapters.GetChapter(userID, p.ChapterID)
	if err != nil {
		return models.Panel{}, mapOwnershipErr(err)
	}

	characters, err := s.assets.ListCharacters(userID, ch.BookID)
	if err != nil {
		return models.Panel{}, mapOwnershipErr(err)
	}
	scenes, err := s.assets.ListScenes(userID, ch.BookID)
	if err != nil {
		return models.Panel{}, mapOwnershipErr(err)
	}
	assets := ai.AssetContext{Characters: characters, Scenes: scenes}

	draft, err := ai.ProcessPanel(context.Background(), s.ai, p.Content, assets)
	if err != nil {
		return models.Panel{}, fmt.Errorf("story: process panel: %w", err)
	}

	updated := applyDraft(p, draft)

	stored, err := s.panels.UpdatePanel(userID, panelID, updated)
	if err != nil {
		return models.Panel{}, mapOwnershipErr(err)
	}
	return stored, nil
}

// loadAssets confirms ownership of chapterID and returns the chapter plus the
// book's asset context. Ownership failures are mapped to ErrNotFound.
func (s *Service) loadAssets(userID, chapterID int64) (models.Chapter, ai.AssetContext, error) {
	ch, err := s.chapters.GetChapter(userID, chapterID)
	if err != nil {
		return models.Chapter{}, ai.AssetContext{}, mapOwnershipErr(err)
	}

	characters, err := s.assets.ListCharacters(userID, ch.BookID)
	if err != nil {
		return models.Chapter{}, ai.AssetContext{}, mapOwnershipErr(err)
	}

	scenes, err := s.assets.ListScenes(userID, ch.BookID)
	if err != nil {
		return models.Chapter{}, ai.AssetContext{}, mapOwnershipErr(err)
	}

	return ch, ai.AssetContext{Characters: characters, Scenes: scenes}, nil
}

// mergePanels merges the model's content-only stage-1 frames with the chapter's
// existing panels, by position and content-equality:
//   - a new content equal to existing[i].Content reuses existing[i] entirely
//     (content, structured fields, image, status);
//   - a changed content becomes a fresh pending frame carrying only the new
//     content (structured fields empty, no image);
//   - extra new frames are appended (content-only, pending);
//   - fewer new frames drop the tail of the existing panels.
//
// ChapterID and a 0-based Order are set on every returned panel. ID is cleared so
// ReplaceForChapter re-inserts the full set atomically.
func mergePanels(chapterID int64, existing []models.Panel, contents []ai.PanelContent) []models.Panel {
	merged := make([]models.Panel, 0, len(contents))
	for i, c := range contents {
		var p models.Panel
		if i < len(existing) && existing[i].Content == c.Content {
			// Unchanged frame: reuse everything (structured fields + image + status).
			p = existing[i]
		} else {
			// New or changed frame: content only, must be (re)parsed and (re)rendered.
			p = models.Panel{Content: c.Content, Status: pendingStatus}
		}
		p.ID = 0
		p.ChapterID = chapterID
		p.Order = i
		merged = append(merged, p)
	}
	return merged
}

// applyDraft maps a sanitized stage-2 draft onto a panel, overwriting its
// structured fields while preserving Content and ImageURL. CharacterIDs is the
// list of present character ids and CharExpressions maps each of those ids to its
// expression.
func applyDraft(p models.Panel, d ai.PanelDraftV2) models.Panel {
	ids := make([]int64, 0, len(d.Characters))
	exprs := make(map[int64]string, len(d.Characters))
	for _, cr := range d.Characters {
		ids = append(ids, int64(cr.ID))
		exprs[int64(cr.ID)] = cr.Expression
	}
	p.Caption = d.Caption
	p.CharacterIDs = ids
	p.SceneID = int64(d.SceneID)
	p.ImagePrompt = d.ImagePrompt
	p.Location = d.Location
	p.Event = d.Event
	p.CharExpressions = exprs
	return p
}

// toAIMsgs converts persisted conversation messages into the ai package's chat
// message shape.
func toAIMsgs(conv []models.ConversationMsg) []ai.Msg {
	msgs := make([]ai.Msg, 0, len(conv))
	for _, m := range conv {
		msgs = append(msgs, ai.Msg{Role: m.Role, Content: m.Content})
	}
	return msgs
}

// mapOwnershipErr normalizes ownership-related errors from collaborators into
// the story package's ErrNotFound, leaving other errors untouched.
func mapOwnershipErr(err error) error {
	if errors.Is(err, chapter.ErrNotFound) ||
		errors.Is(err, asset.ErrNotFound) ||
		errors.Is(err, panel.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
