package book

import (
	"errors"
	"fmt"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ErrValidation is returned when caller-supplied input fails validation (for
// example, an empty title). HTTP handlers map it to 400 Bad Request.
var ErrValidation = errors.New("validation failed")

// Service implements the book use cases on top of a Repo. It validates input at
// the boundary and delegates persistence and ownership scoping to the Repo.
type Service struct {
	repo *Repo
}

// NewService wires a Service to its Repo.
func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Create validates and creates a book for userID. The title is trimmed and must
// be non-empty; otherwise ErrValidation is returned.
func (s *Service) Create(userID int64, title, style, summary string) (models.Book, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return models.Book{}, fmt.Errorf("%w: 标题不能为空", ErrValidation)
	}
	return s.repo.Create(userID, title, style, summary)
}

// List returns all books owned by userID, most recently created first.
func (s *Service) List(userID int64) ([]models.Book, error) {
	return s.repo.ListByUser(userID)
}

// Get returns the book owned by userID, or ErrNotFound.
func (s *Service) Get(userID, bookID int64) (models.Book, error) {
	return s.repo.Get(userID, bookID)
}

// Update validates and updates the book owned by userID. The title is trimmed
// and must be non-empty. It returns ErrNotFound if the book is not owned by the
// user.
func (s *Service) Update(userID, bookID int64, title, style, summary string) (models.Book, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return models.Book{}, fmt.Errorf("%w: 标题不能为空", ErrValidation)
	}
	return s.repo.Update(userID, bookID, title, style, summary)
}

// Delete removes the book owned by userID, or returns ErrNotFound.
func (s *Service) Delete(userID, bookID int64) error {
	return s.repo.Delete(userID, bookID)
}
