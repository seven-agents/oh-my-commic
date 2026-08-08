package chapter

import (
	"errors"

	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// BookOwner is the narrow ownership gate the Service depends on. It is satisfied
// by *book.Repo. Defining it here (where it is used) keeps the chapter package
// free of a hard dependency on book internals.
type BookOwner interface {
	// Get returns the book with bookID owned by userID, or book.ErrNotFound if
	// the book does not exist or belongs to another user.
	Get(userID, bookID int64) (models.Book, error)
}

// allowedTransitions defines the chapter status state machine: each key maps to
// the set of statuses it may advance (or revert) to. A (from, to) pair absent
// from this map is rejected with ErrInvalidStatus. Setting a chapter to the
// status it already has is always allowed (idempotent no-op, handled in
// SetStatus) and need not appear here.
//
// This is a creative, iterative tool: users freely jump between storyboarding
// (chatting), rendering (drawing panels) and done (saving the book), and back
// again to revise. The transitions are therefore permissive — the machine only
// exists to keep the status badge meaningful, not to enforce a rigid pipeline.
//
//	draft         → storyboarding
//	storyboarding → rendering, draft, done   (保存成书 may skip the rendering marker)
//	rendering     → done, storyboarding
//	done          → rendering, storyboarding (re-open a finished chapter to revise)
var allowedTransitions = map[string]map[string]bool{
	"draft":         {"storyboarding": true},
	"storyboarding": {"rendering": true, "draft": true, "done": true},
	"rendering":     {"done": true, "storyboarding": true},
	"done":          {"rendering": true, "storyboarding": true},
}

// Service implements the chapter use cases on top of a Repo. Every operation
// first confirms, via the BookOwner gate, that the target book belongs to the
// calling user; any ownership failure is mapped to ErrNotFound so the existence
// of another user's books and chapters never leaks.
type Service struct {
	repo  *Repo
	books BookOwner
}

// NewService wires a Service to its Repo and the book ownership gate.
func NewService(repo *Repo, books BookOwner) *Service {
	return &Service{repo: repo, books: books}
}

// ownBook confirms userID owns bookID, translating book.ErrNotFound into the
// chapter package's ErrNotFound.
func (s *Service) ownBook(userID, bookID int64) error {
	if _, err := s.books.Get(userID, bookID); err != nil {
		if errors.Is(err, book.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// CreateChapter creates a chapter under bookID after verifying userID owns the
// book. Cross-user or unknown books return ErrNotFound.
func (s *Service) CreateChapter(userID, bookID int64, title string) (models.Chapter, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return models.Chapter{}, err
	}
	return s.repo.Create(bookID, title)
}

// EnsureCover returns the book's single cover chapter, creating it if absent,
// after verifying userID owns the book. A book has at most one cover chapter, so
// a repeat call returns the same chapter. Cross-user or unknown books return
// ErrNotFound.
func (s *Service) EnsureCover(userID, bookID int64) (models.Chapter, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return models.Chapter{}, err
	}
	existing, err := s.repo.FindCover(bookID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return models.Chapter{}, err
	}
	return s.repo.CreateCover(bookID)
}

// ListChapters returns all chapters under bookID after verifying userID owns the
// book. Cross-user or unknown books return ErrNotFound.
func (s *Service) ListChapters(userID, bookID int64) ([]models.Chapter, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return nil, err
	}
	return s.repo.ListByBook(bookID)
}

// GetChapter returns the chapter with chapterID after re-checking that its
// owning book belongs to userID. Cross-user access returns ErrNotFound.
func (s *Service) GetChapter(userID, chapterID int64) (models.Chapter, error) {
	c, err := s.repo.Get(chapterID)
	if err != nil {
		return models.Chapter{}, err
	}
	if err := s.ownBook(userID, c.BookID); err != nil {
		return models.Chapter{}, err
	}
	return c, nil
}

// SetStatus advances the chapter with chapterID to status after verifying
// ownership and validating the transition against allowedTransitions. It returns
// ErrNotFound for cross-user or unknown chapters and ErrInvalidStatus for an
// illegal transition.
func (s *Service) SetStatus(userID, chapterID int64, status string) (models.Chapter, error) {
	c, err := s.repo.Get(chapterID)
	if err != nil {
		return models.Chapter{}, err
	}
	if err := s.ownBook(userID, c.BookID); err != nil {
		return models.Chapter{}, err
	}
	// Setting the status it already has is an idempotent no-op — never an error.
	if c.Status == status {
		return c, nil
	}
	if !allowedTransitions[c.Status][status] {
		return models.Chapter{}, ErrInvalidStatus
	}
	return s.repo.SetStatus(chapterID, status)
}

// Delete removes the chapter with chapterID after re-checking that its owning
// book belongs to userID. Deleting a chapter cascades to its panels at the
// database level. Cross-user or unknown chapters return ErrNotFound.
func (s *Service) Delete(userID, chapterID int64) error {
	c, err := s.repo.Get(chapterID)
	if err != nil {
		return err
	}
	if err := s.ownBook(userID, c.BookID); err != nil {
		return err
	}
	return s.repo.Delete(chapterID)
}

// SetSummary overwrites the AI-polished story summary of the chapter with
// chapterID after re-checking that its owning book belongs to userID. Cross-user
// or unknown chapters return ErrNotFound.
func (s *Service) SetSummary(userID, chapterID int64, summary string) (models.Chapter, error) {
	c, err := s.repo.Get(chapterID)
	if err != nil {
		return models.Chapter{}, err
	}
	if err := s.ownBook(userID, c.BookID); err != nil {
		return models.Chapter{}, err
	}
	return s.repo.SetSummary(chapterID, summary)
}

// SaveConversation persists the stage-1 storyboard-chat history and the target
// panel_count of the chapter with chapterID after re-checking that its owning
// book belongs to userID. Cross-user or unknown chapters return ErrNotFound.
func (s *Service) SaveConversation(userID, chapterID int64, conv []models.ConversationMsg, panelCount int) (models.Chapter, error) {
	c, err := s.repo.Get(chapterID)
	if err != nil {
		return models.Chapter{}, err
	}
	if err := s.ownBook(userID, c.BookID); err != nil {
		return models.Chapter{}, err
	}
	return s.repo.SetConversation(chapterID, conv, panelCount)
}
