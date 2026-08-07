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

// Converse verifies chapter ownership, loads the book's assets, and runs one
// turn of storyboard discussion. Cross-user or unknown chapters yield
// ErrNotFound.
func (s *Service) Converse(userID, chapterID int64, history []ai.Msg) (string, error) {
	assets, err := s.loadAssets(userID, chapterID)
	if err != nil {
		return "", err
	}

	reply, err := ai.Converse(context.Background(), s.ai, history, assets)
	if err != nil {
		return "", fmt.Errorf("story: converse: %w", err)
	}
	return reply, nil
}

// GenerateStoryboard verifies ownership, loads assets, asks the model for n
// panels, persists them via ReplacePanels, advances the chapter to
// storyboarding, and returns the stored panels. Cross-user or unknown chapters
// yield ErrNotFound.
func (s *Service) GenerateStoryboard(userID, chapterID int64, history []ai.Msg, n int) ([]models.Panel, error) {
	assets, err := s.loadAssets(userID, chapterID)
	if err != nil {
		return nil, err
	}

	drafts, err := ai.GenStoryboard(context.Background(), s.ai, history, assets, n)
	if err != nil {
		return nil, fmt.Errorf("story: generate storyboard: %w", err)
	}

	mapped := draftsToPanels(chapterID, drafts)

	stored, err := s.panels.ReplacePanels(userID, chapterID, mapped)
	if err != nil {
		return nil, mapOwnershipErr(err)
	}

	if _, err := s.chapters.SetStatus(userID, chapterID, storyboardingStatus); err != nil {
		return nil, mapOwnershipErr(err)
	}

	return stored, nil
}

// loadAssets confirms ownership of chapterID and returns the book's asset
// context. Ownership failures are mapped to ErrNotFound.
func (s *Service) loadAssets(userID, chapterID int64) (ai.AssetContext, error) {
	ch, err := s.chapters.GetChapter(userID, chapterID)
	if err != nil {
		return ai.AssetContext{}, mapOwnershipErr(err)
	}

	characters, err := s.assets.ListCharacters(userID, ch.BookID)
	if err != nil {
		return ai.AssetContext{}, mapOwnershipErr(err)
	}

	scenes, err := s.assets.ListScenes(userID, ch.BookID)
	if err != nil {
		return ai.AssetContext{}, mapOwnershipErr(err)
	}

	return ai.AssetContext{Characters: characters, Scenes: scenes}, nil
}

// draftsToPanels maps model drafts to persistable panels, all in pending status.
func draftsToPanels(chapterID int64, drafts []ai.PanelDraft) []models.Panel {
	panels := make([]models.Panel, 0, len(drafts))
	for _, d := range drafts {
		panels = append(panels, models.Panel{
			ChapterID:    chapterID,
			Caption:      d.Caption,
			CharacterIDs: d.CharacterIDs,
			SceneID:      d.SceneID,
			ImagePrompt:  d.ImagePrompt,
			Status:       pendingStatus,
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
