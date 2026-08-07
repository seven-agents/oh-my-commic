// Package story orchestrates AI-driven storyboard conversation and generation:
// it verifies chapter ownership, loads the book's asset context, drives the ai
// gateway, and persists the resulting panels.
package story

import (
	"context"
	"errors"
	"fmt"

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

// StoryboardChat runs one turn of the unified conversational storyboard flow: it
// verifies chapter ownership, loads the book's assets, asks the model for a
// {reply, panels} result, persists the structured panels via ReplacePanels,
// advances the chapter to storyboarding, and returns the model's reply plus the
// stored panels. panelCount is a soft target for how many frames to produce
// (0 lets the prompt pick a default range). Cross-user or unknown chapters yield
// ErrNotFound; an AI or parse failure is returned as a non-ErrNotFound error so
// nothing half-baked is stored.
func (s *Service) StoryboardChat(userID, chapterID int64, history []ai.Msg, panelCount int) (string, []models.Panel, error) {
	ch, assets, err := s.loadAssets(userID, chapterID)
	if err != nil {
		return "", nil, err
	}

	res, err := ai.StoryboardChat(context.Background(), s.ai, history, assets, panelCount)
	if err != nil {
		return "", nil, fmt.Errorf("story: storyboard chat: %w", err)
	}

	mapped := draftsToPanels(chapterID, res.Panels)

	stored, err := s.panels.ReplacePanels(userID, chapterID, mapped)
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

	return res.Reply, stored, nil
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

// draftsToPanels maps the model's structured drafts to persistable panels, all
// in pending status. The ai layer has already sanitized every draft against the
// book's assets (dropping hallucinated character/scene ids and enforcing the ≤3
// reference cap), so here we simply flatten each draft: CharacterIDs is the list
// of present character ids and CharExpressions maps each of those ids to its
// expression.
func draftsToPanels(chapterID int64, drafts []ai.PanelDraftV2) []models.Panel {
	panels := make([]models.Panel, 0, len(drafts))
	for _, d := range drafts {
		ids := make([]int64, 0, len(d.Characters))
		exprs := make(map[int64]string, len(d.Characters))
		for _, cr := range d.Characters {
			ids = append(ids, cr.ID)
			exprs[cr.ID] = cr.Expression
		}
		panels = append(panels, models.Panel{
			ChapterID:       chapterID,
			Caption:         d.Caption,
			CharacterIDs:    ids,
			SceneID:         d.SceneID,
			ImagePrompt:     d.ImagePrompt,
			Location:        d.Location,
			Event:           d.Event,
			CharExpressions: exprs,
			Status:          pendingStatus,
		})
	}
	return panels
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
