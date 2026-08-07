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
// from this map is rejected with ErrInvalidStatus. For example draft → done is
// not present and is therefore illegal.
//
//	draft         → storyboarding
//	storyboarding → rendering, draft
//	rendering     → done, storyboarding
//	done          → rendering
var allowedTransitions = map[string]map[string]bool{
	"draft":         {"storyboarding": true},
	"storyboarding": {"rendering": true, "draft": true},
	"rendering":     {"done": true, "storyboarding": true},
	"done":          {"rendering": true},
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
	if !allowedTransitions[c.Status][status] {
		return models.Chapter{}, ErrInvalidStatus
	}
	return s.repo.SetStatus(chapterID, status)
}
