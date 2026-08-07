package asset

import (
	"errors"

	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// BookOwner is the narrow ownership gate the Service depends on. It is satisfied
// by *book.Repo. Defining it here (where it is used) keeps the asset package
// free of a hard dependency on book internals.
type BookOwner interface {
	// Get returns the book with bookID owned by userID, or book.ErrNotFound if
	// the book does not exist or belongs to another user.
	Get(userID, bookID int64) (models.Book, error)
}

// Service implements the character and scene use cases on top of a Repo. Every
// operation first confirms, via the BookOwner gate, that the target book belongs
// to the calling user; any ownership failure is mapped to ErrNotFound so the
// existence of another user's books and assets never leaks.
type Service struct {
	repo  *Repo
	books BookOwner
}

// NewService wires a Service to its Repo and the book ownership gate.
func NewService(repo *Repo, books BookOwner) *Service {
	return &Service{repo: repo, books: books}
}

// ownBook confirms userID owns bookID, translating book.ErrNotFound into the
// asset package's ErrNotFound.
func (s *Service) ownBook(userID, bookID int64) error {
	if _, err := s.books.Get(userID, bookID); err != nil {
		if errors.Is(err, book.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// CreateCharacter creates a character under bookID after verifying userID owns
// the book. The character's BookID is forced to bookID regardless of the input.
func (s *Service) CreateCharacter(userID, bookID int64, c models.Character) (models.Character, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return models.Character{}, err
	}
	c.BookID = bookID
	return s.repo.CreateCharacter(bookID, c)
}

// ListCharacters returns all characters under bookID after verifying userID owns
// the book.
func (s *Service) ListCharacters(userID, bookID int64) ([]models.Character, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return nil, err
	}
	return s.repo.ListCharacters(bookID)
}

// UpdateCharacter updates the character with characterID after re-checking that
// its owning book belongs to userID. Cross-user access returns ErrNotFound.
func (s *Service) UpdateCharacter(userID, characterID int64, c models.Character) (models.Character, error) {
	existing, err := s.repo.GetCharacter(characterID)
	if err != nil {
		return models.Character{}, err
	}
	if err := s.ownBook(userID, existing.BookID); err != nil {
		return models.Character{}, err
	}
	return s.repo.UpdateCharacter(characterID, c)
}

// DeleteCharacter deletes the character with characterID after re-checking that
// its owning book belongs to userID. Cross-user access returns ErrNotFound.
func (s *Service) DeleteCharacter(userID, characterID int64) error {
	existing, err := s.repo.GetCharacter(characterID)
	if err != nil {
		return err
	}
	if err := s.ownBook(userID, existing.BookID); err != nil {
		return err
	}
	return s.repo.DeleteCharacter(characterID)
}

// CreateScene creates a scene under bookID after verifying userID owns the book.
// The scene's BookID is forced to bookID regardless of the input.
func (s *Service) CreateScene(userID, bookID int64, sc models.Scene) (models.Scene, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return models.Scene{}, err
	}
	sc.BookID = bookID
	return s.repo.CreateScene(bookID, sc)
}

// ListScenes returns all scenes under bookID after verifying userID owns the
// book.
func (s *Service) ListScenes(userID, bookID int64) ([]models.Scene, error) {
	if err := s.ownBook(userID, bookID); err != nil {
		return nil, err
	}
	return s.repo.ListScenes(bookID)
}

// UpdateScene updates the scene with sceneID after re-checking that its owning
// book belongs to userID. Cross-user access returns ErrNotFound.
func (s *Service) UpdateScene(userID, sceneID int64, sc models.Scene) (models.Scene, error) {
	existing, err := s.repo.GetScene(sceneID)
	if err != nil {
		return models.Scene{}, err
	}
	if err := s.ownBook(userID, existing.BookID); err != nil {
		return models.Scene{}, err
	}
	return s.repo.UpdateScene(sceneID, sc)
}

// DeleteScene deletes the scene with sceneID after re-checking that its owning
// book belongs to userID. Cross-user access returns ErrNotFound.
func (s *Service) DeleteScene(userID, sceneID int64) error {
	existing, err := s.repo.GetScene(sceneID)
	if err != nil {
		return err
	}
	if err := s.ownBook(userID, existing.BookID); err != nil {
		return err
	}
	return s.repo.DeleteScene(sceneID)
}
