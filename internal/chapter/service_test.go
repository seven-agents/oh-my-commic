package chapter

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newTestService opens an in-memory database, seeds two users (ids 1 and 2 —
// required because books.user_id has a foreign key to users), and returns a
// chapter Service wired to a real book.Repo as its ownership gate plus the
// book.Repo itself for seeding books. The DB is closed when the test finishes.
func newTestService(t *testing.T) (*Service, *book.Repo) {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	seedUsers(t, d, 2)

	bookRepo := book.NewRepo(d)
	svc := NewService(NewRepo(d), bookRepo)
	return svc, bookRepo
}

// seedUsers inserts n users so books referencing user ids 1..n satisfy the FK.
func seedUsers(t *testing.T, d *sql.DB, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := d.Exec(
			`INSERT INTO users (nickname, password_hash) VALUES (?, ?)`,
			"user"+string(rune('0'+i)), "hash",
		); err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}
}

// TestCreateAndListOrderAutoIncrements verifies that chapters created under a
// book receive consecutive order values.
func TestCreateAndListOrderAutoIncrements(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}

	c1, err := svc.CreateChapter(1, b.ID, "第一章")
	if err != nil {
		t.Fatalf("create chapter 1: %v", err)
	}
	c2, err := svc.CreateChapter(1, b.ID, "第二章")
	if err != nil {
		t.Fatalf("create chapter 2: %v", err)
	}

	if c1.Order != 1 {
		t.Fatalf("首章 order 应为 1, got %d", c1.Order)
	}
	if c2.Order != 2 {
		t.Fatalf("第二章 order 应为 2, got %d", c2.Order)
	}
	if c1.Status != "draft" {
		t.Fatalf("新章节状态应为 draft, got %q", c1.Status)
	}

	list, err := svc.ListChapters(1, b.ID)
	if err != nil {
		t.Fatalf("list chapters: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("应有 2 个章节, got %d", len(list))
	}
	if list[0].Order != 1 || list[1].Order != 2 {
		t.Fatalf("章节应按 order 升序: %+v", list)
	}
}

// TestLegalStatusFlow walks the full happy-path transition chain
// draft → storyboarding → rendering → done.
func TestLegalStatusFlow(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := svc.CreateChapter(1, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	for _, want := range []string{"storyboarding", "rendering", "done"} {
		updated, err := svc.SetStatus(1, c.ID, want)
		if err != nil {
			t.Fatalf("transition to %q failed: %v", want, err)
		}
		if updated.Status != want {
			t.Fatalf("状态应为 %q, got %q", want, updated.Status)
		}
	}
}

// TestIllegalStatusTransition verifies that a transition absent from the state
// machine (draft → done) is rejected with ErrInvalidStatus and does not mutate
// the stored status.
func TestIllegalStatusTransition(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := svc.CreateChapter(1, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	if _, err := svc.SetStatus(1, c.ID, "done"); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("draft→done 应返回 ErrInvalidStatus, got %v", err)
	}

	got, err := svc.GetChapter(1, c.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.Status != "draft" {
		t.Fatalf("非法流转后状态应仍为 draft, got %q", got.Status)
	}
}

// TestCrossUserIsolation is the most important test: chapters under user 1's
// book must be invisible and inaccessible to user 2, and every cross-user
// operation must return ErrNotFound rather than leaking existence.
func TestCrossUserIsolation(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := svc.CreateChapter(1, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	// User 2 creating a chapter under user 1's book.
	if _, err := svc.CreateChapter(2, b.ID, "越权"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户创建应返回 ErrNotFound, got %v", err)
	}
	// User 2 listing user 1's chapters.
	if _, err := svc.ListChapters(2, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户列出应返回 ErrNotFound, got %v", err)
	}
	// User 2 reading user 1's chapter.
	if _, err := svc.GetChapter(2, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户读取应返回 ErrNotFound, got %v", err)
	}
	// User 2 changing status of user 1's chapter.
	if _, err := svc.SetStatus(2, c.ID, "storyboarding"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户改状态应返回 ErrNotFound, got %v", err)
	}
}

// TestGetSetStatusUnknownChapter verifies that operations on a nonexistent
// chapter id return ErrNotFound.
func TestGetSetStatusUnknownChapter(t *testing.T) {
	svc, _ := newTestService(t)

	if _, err := svc.GetChapter(1, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知章节 Get 应返回 ErrNotFound, got %v", err)
	}
	if _, err := svc.SetStatus(1, 999, "storyboarding"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知章节 SetStatus 应返回 ErrNotFound, got %v", err)
	}
}
