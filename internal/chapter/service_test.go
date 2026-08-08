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

// TestEnsureCoverIsSingletonWithOrderZero verifies EnsureCover creates exactly
// one cover chapter (order 0, is_cover true, title "封面") and that calling it
// again returns the SAME chapter rather than creating a second one.
func TestEnsureCoverIsSingletonWithOrderZero(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}

	first, err := svc.EnsureCover(1, b.ID)
	if err != nil {
		t.Fatalf("ensure cover (create): %v", err)
	}
	if first.Order != 0 {
		t.Fatalf("封面章 order 应为 0, got %d", first.Order)
	}
	if !first.IsCover {
		t.Fatalf("封面章 IsCover 应为 true, got false")
	}
	if first.Title != "封面" {
		t.Fatalf("封面章标题应为 封面, got %q", first.Title)
	}
	if first.Status != "draft" {
		t.Fatalf("封面章状态应为 draft, got %q", first.Status)
	}

	second, err := svc.EnsureCover(1, b.ID)
	if err != nil {
		t.Fatalf("ensure cover (find): %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("重复调用应返回同一封面章, first=%d second=%d", first.ID, second.ID)
	}

	// Only one cover chapter exists in the book's chapter list.
	list, err := svc.ListChapters(1, b.ID)
	if err != nil {
		t.Fatalf("list chapters: %v", err)
	}
	covers := 0
	for _, c := range list {
		if c.IsCover {
			covers++
		}
	}
	if covers != 1 {
		t.Fatalf("每本书应只有 1 个封面章, got %d", covers)
	}
}

// TestEnsureCoverCrossUser verifies user 2 cannot create/find a cover chapter on
// user 1's book; the ownership gate surfaces ErrNotFound.
func TestEnsureCoverCrossUser(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	if _, err := svc.EnsureCover(2, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户 EnsureCover 应返回 ErrNotFound, got %v", err)
	}
	if _, err := svc.EnsureCover(1, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知书籍 EnsureCover 应返回 ErrNotFound, got %v", err)
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

// TestSetStatusIdempotentAndSaveBook covers the "保存成书" flow: storyboarding →
// done is legal (it may skip the rendering marker), and setting a status to the
// one it already has is an idempotent no-op rather than an error.
func TestSetStatusIdempotentAndSaveBook(t *testing.T) {
	svc, books := newTestService(t)
	b, _ := books.Create(1, "书", "ghibli", "")
	c, _ := svc.CreateChapter(1, b.ID, "章")

	if _, err := svc.SetStatus(1, c.ID, "storyboarding"); err != nil {
		t.Fatalf("draft→storyboarding failed: %v", err)
	}
	// 保存成书: storyboarding → done directly.
	if _, err := svc.SetStatus(1, c.ID, "done"); err != nil {
		t.Fatalf("storyboarding→done (保存成书) should be legal, got %v", err)
	}
	// Idempotent: setting done again is a no-op, not ErrInvalidStatus.
	got, err := svc.SetStatus(1, c.ID, "done")
	if err != nil {
		t.Fatalf("done→done should be an idempotent no-op, got %v", err)
	}
	if got.Status != "done" {
		t.Fatalf("status should stay done, got %q", got.Status)
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
	if _, err := svc.SetSummary(1, 999, "概述"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知章节 SetSummary 应返回 ErrNotFound, got %v", err)
	}
}

// TestSetSummaryOwnershipAndRoundTrip verifies the owner can set a chapter's
// summary, that a cross-user attempt returns ErrNotFound without mutating the
// stored summary, and that the value round-trips through Get and ListChapters.
func TestSetSummaryOwnershipAndRoundTrip(t *testing.T) {
	svc, books := newTestService(t)
	b, err := books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := svc.CreateChapter(1, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	if c.Summary != "" {
		t.Fatalf("新章节 summary 应为空, got %q", c.Summary)
	}

	// Cross-user write must be rejected and must not mutate the summary.
	if _, err := svc.SetSummary(2, c.ID, "越权概述"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户 SetSummary 应返回 ErrNotFound, got %v", err)
	}
	if got, _ := svc.GetChapter(1, c.ID); got.Summary != "" {
		t.Fatalf("跨用户尝试后 summary 不应被改动, got %q", got.Summary)
	}

	// Owner sets the summary.
	const summary = "小狐狸在黄昏的森林里找到了回家的路，温暖又勇敢。"
	updated, err := svc.SetSummary(1, c.ID, summary)
	if err != nil {
		t.Fatalf("owner SetSummary: %v", err)
	}
	if updated.Summary != summary {
		t.Fatalf("SetSummary 返回值 summary 错: %q", updated.Summary)
	}

	// Round-trips through Get.
	got, err := svc.GetChapter(1, c.ID)
	if err != nil {
		t.Fatalf("get chapter: %v", err)
	}
	if got.Summary != summary {
		t.Fatalf("Get 的 summary 应往返, got %q", got.Summary)
	}

	// Round-trips through ListChapters.
	list, err := svc.ListChapters(1, b.ID)
	if err != nil {
		t.Fatalf("list chapters: %v", err)
	}
	if len(list) != 1 || list[0].Summary != summary {
		t.Fatalf("ListChapters 的 summary 应往返, got %+v", list)
	}
}
