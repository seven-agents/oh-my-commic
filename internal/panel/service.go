package panel

import (
	"errors"

	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ChapterOwner is the narrow ownership gate the Service depends on. It is
// satisfied by *chapter.Service. Defining it here (where it is used) keeps the
// panel package free of a hard dependency on chapter internals. GetChapter
// returns chapter.ErrNotFound when the chapter does not exist or belongs to
// another user, which the Service translates into the panel package's
// ErrNotFound.
type ChapterOwner interface {
	// GetChapter returns the chapter with chapterID owned (transitively via its
	// book) by userID, or chapter.ErrNotFound.
	GetChapter(userID, chapterID int64) (models.Chapter, error)
}

// Service implements the panel use cases on top of a Repo. Every operation first
// confirms, via the ChapterOwner gate, that the target chapter belongs to the
// calling user; any ownership failure is mapped to ErrNotFound so the existence
// of another user's chapters and panels never leaks. Per-panel operations first
// load the panel to resolve its real chapter, then re-check ownership of that
// chapter.
type Service struct {
	repo     *Repo
	chapters ChapterOwner
}

// NewService wires a Service to its Repo and the chapter ownership gate.
func NewService(repo *Repo, chapters ChapterOwner) *Service {
	return &Service{repo: repo, chapters: chapters}
}

// ownChapter confirms userID owns chapterID, translating chapter.ErrNotFound
// into the panel package's ErrNotFound.
func (s *Service) ownChapter(userID, chapterID int64) error {
	if _, err := s.chapters.GetChapter(userID, chapterID); err != nil {
		if errors.Is(err, chapter.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// ReplacePanels atomically replaces every panel of chapterID after verifying
// userID owns the chapter. Cross-user or unknown chapters return ErrNotFound.
func (s *Service) ReplacePanels(userID, chapterID int64, panels []models.Panel) ([]models.Panel, error) {
	if err := s.ownChapter(userID, chapterID); err != nil {
		return nil, err
	}
	return s.repo.ReplaceForChapter(chapterID, panels)
}

// ListPanels returns all panels under chapterID after verifying userID owns the
// chapter. Cross-user or unknown chapters return ErrNotFound.
func (s *Service) ListPanels(userID, chapterID int64) ([]models.Panel, error) {
	if err := s.ownChapter(userID, chapterID); err != nil {
		return nil, err
	}
	return s.repo.ListByChapter(chapterID)
}

// UpdatePanel updates the editable fields of panelID after loading it to resolve
// its owning chapter and verifying userID owns that chapter. Cross-user or
// unknown panels return ErrNotFound.
func (s *Service) UpdatePanel(userID, panelID int64, p models.Panel) (models.Panel, error) {
	existing, err := s.repo.Get(panelID)
	if err != nil {
		return models.Panel{}, err
	}
	if err := s.ownChapter(userID, existing.ChapterID); err != nil {
		return models.Panel{}, err
	}
	return s.repo.Update(panelID, p)
}

// GetPanel returns the panel with panelID after loading it to resolve its owning
// chapter and verifying userID owns that chapter. Cross-user or unknown panels
// return ErrNotFound.
func (s *Service) GetPanel(userID, panelID int64) (models.Panel, error) {
	existing, err := s.repo.Get(panelID)
	if err != nil {
		return models.Panel{}, err
	}
	if err := s.ownChapter(userID, existing.ChapterID); err != nil {
		return models.Panel{}, err
	}
	return existing, nil
}

// SetPanelImage stores url as panelID's image after verifying ownership of its
// chapter. Cross-user or unknown panels return ErrNotFound.
func (s *Service) SetPanelImage(userID, panelID int64, url string) (models.Panel, error) {
	existing, err := s.repo.Get(panelID)
	if err != nil {
		return models.Panel{}, err
	}
	if err := s.ownChapter(userID, existing.ChapterID); err != nil {
		return models.Panel{}, err
	}
	return s.repo.SetImage(panelID, url)
}

// SetPanelStatus overwrites panelID's status after verifying ownership of its
// chapter. Cross-user or unknown panels return ErrNotFound.
func (s *Service) SetPanelStatus(userID, panelID int64, status string) (models.Panel, error) {
	existing, err := s.repo.Get(panelID)
	if err != nil {
		return models.Panel{}, err
	}
	if err := s.ownChapter(userID, existing.ChapterID); err != nil {
		return models.Panel{}, err
	}
	return s.repo.SetStatus(panelID, status)
}
