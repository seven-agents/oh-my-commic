package asset

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// newServiceTestEnv opens an in-memory DB, seeds two users, and returns a wired
// asset Service plus the book repo used to seed owned books.
func newServiceTestEnv(t *testing.T) (*Service, *book.Repo) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	seedHandlerUsers(t, d, 2)

	books := book.NewRepo(d)
	return NewService(NewRepo(d), books), books
}

func TestGetCharacterOwnerAndCrossUser(t *testing.T) {
	svc, books := newServiceTestEnv(t)
	b, _ := books.Create(1, "书", "ghibli", "")

	created, err := svc.CreateCharacter(1, b.ID, models.Character{Name: "狐狸"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.GetCharacter(1, created.ID)
	if err != nil {
		t.Fatalf("owner GetCharacter: %v", err)
	}
	if got.Name != "狐狸" {
		t.Fatalf("wrong character: %+v", got)
	}

	// Cross-user access is indistinguishable from not-found.
	if _, err := svc.GetCharacter(2, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user GetCharacter should be ErrNotFound, got %v", err)
	}
	// Unknown id → ErrNotFound.
	if _, err := svc.GetCharacter(1, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown GetCharacter should be ErrNotFound, got %v", err)
	}
}

func TestGetSceneOwnerAndCrossUser(t *testing.T) {
	svc, books := newServiceTestEnv(t)
	b, _ := books.Create(1, "书", "ghibli", "")

	created, err := svc.CreateScene(1, b.ID, models.Scene{Name: "森林"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.GetScene(1, created.ID)
	if err != nil {
		t.Fatalf("owner GetScene: %v", err)
	}
	if got.Name != "森林" {
		t.Fatalf("wrong scene: %+v", got)
	}

	if _, err := svc.GetScene(2, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-user GetScene should be ErrNotFound, got %v", err)
	}
	if _, err := svc.GetScene(1, 999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown GetScene should be ErrNotFound, got %v", err)
	}
}
