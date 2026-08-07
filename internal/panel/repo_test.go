package panel

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/chapter"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// panelTestEnv bundles a panel Service wired to a real chapter.Service ownership
// gate, plus helpers for seeding the full ownership chain (user → book →
// chapter) required by the panels foreign key.
type panelTestEnv struct {
	panels   *Service
	chapters *chapter.Service
	books    *book.Repo
	db       *sql.DB
}

// newPanelTestEnv opens an in-memory database, seeds two users (ids 1 and 2 —
// required because books.user_id has a foreign key to users), and returns an
// environment whose panel Service is gated by a real chapter.Service. The DB is
// closed when the test finishes.
func newPanelTestEnv(t *testing.T) *panelTestEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	seedUsers(t, d, 2)

	bookRepo := book.NewRepo(d)
	chapterSvc := chapter.NewService(chapter.NewRepo(d), bookRepo)
	panelSvc := NewService(NewRepo(d), chapterSvc)

	return &panelTestEnv{
		panels:   panelSvc,
		chapters: chapterSvc,
		books:    bookRepo,
		db:       d,
	}
}

// newChapter seeds a book owned by userID and a chapter under it, returning the
// chapter.
func (e *panelTestEnv) newChapter(t *testing.T, userID int64) models.Chapter {
	t.Helper()
	b, err := e.books.Create(userID, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	c, err := e.chapters.CreateChapter(userID, b.ID, "章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	return c
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

// TestReplacePanelsReorders verifies that ReplacePanels assigns 0-based order
// values in slice order and that CharacterIDs round-trips through the JSON TEXT
// column intact.
func TestReplacePanelsReorders(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)

	in := []models.Panel{
		{
			Caption:         "A",
			CharacterIDs:    []int64{2, 3},
			Location:        "森林",
			Event:           "出发",
			CharExpressions: map[int64]string{2: "开心", 3: "好奇"},
		},
		{Caption: "B"},
	}
	out, err := env.panels.ReplacePanels(1, ch.ID, in)
	if err != nil {
		t.Fatalf("replace panels: %v", err)
	}
	if out[0].Order != 0 || out[1].Order != 1 {
		t.Fatalf("order 未重排: %+v", out)
	}

	got, err := env.panels.ListPanels(1, ch.ID)
	if err != nil {
		t.Fatalf("list panels: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应有 2 个分镜, got %d", len(got))
	}
	if got[0].Order != 0 || got[1].Order != 1 {
		t.Fatalf("列出顺序应按 order: %+v", got)
	}
	if len(got[0].CharacterIDs) != 2 || got[0].CharacterIDs[0] != 2 || got[0].CharacterIDs[1] != 3 {
		t.Fatalf("CharacterIDs JSON 往返错: %+v", got[0])
	}
	// Structured fields must round-trip through the new columns.
	if got[0].Location != "森林" || got[0].Event != "出发" {
		t.Fatalf("location/event 往返错: %+v", got[0])
	}
	if got[0].CharExpressions[2] != "开心" || got[0].CharExpressions[3] != "好奇" {
		t.Fatalf("CharExpressions JSON 往返错: %+v", got[0].CharExpressions)
	}
	// Panel B was inserted with no character ids; it must round-trip to an empty
	// (non-nil) slice, never nil.
	if got[1].CharacterIDs == nil {
		t.Fatalf("空 CharacterIDs 应为非 nil 空切片, got nil")
	}
	// A panel with no expressions must round-trip to an empty (non-nil) map.
	if got[1].CharExpressions == nil {
		t.Fatalf("空 CharExpressions 应为非 nil 空 map, got nil")
	}
	if got[0].Status != statusPending {
		t.Fatalf("新分镜状态应为 pending, got %q", got[0].Status)
	}
}

// TestReplaceReplacesAll verifies that a second ReplacePanels fully supplants the
// prior panel set (delete-then-insert), leaving exactly the new panels.
func TestReplaceReplacesAll(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)

	if _, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{
		{Caption: "old1"}, {Caption: "old2"}, {Caption: "old3"},
	}); err != nil {
		t.Fatalf("first replace: %v", err)
	}
	out, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{{Caption: "new1"}})
	if err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if len(out) != 1 || out[0].Caption != "new1" || out[0].Order != 0 {
		t.Fatalf("second replace should supplant all: %+v", out)
	}

	got, err := env.panels.ListPanels(1, ch.ID)
	if err != nil {
		t.Fatalf("list panels: %v", err)
	}
	if len(got) != 1 || got[0].Caption != "new1" {
		t.Fatalf("章节应仅剩替换后的分镜: %+v", got)
	}
}

// TestUpdatePanel verifies the editable fields are persisted and returned.
func TestUpdatePanel(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)

	out, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{{Caption: "orig"}})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	id := out[0].ID

	updated, err := env.panels.UpdatePanel(1, id, models.Panel{
		Caption:         "edited",
		CharacterIDs:    []int64{7},
		SceneID:         5,
		ImagePrompt:     "a prompt",
		Location:        "山顶",
		Event:           "看日出",
		CharExpressions: map[int64]string{7: "惊喜"},
	})
	if err != nil {
		t.Fatalf("update panel: %v", err)
	}
	if updated.Caption != "edited" || updated.SceneID != 5 || updated.ImagePrompt != "a prompt" {
		t.Fatalf("editable fields not persisted: %+v", updated)
	}
	if updated.Location != "山顶" || updated.Event != "看日出" {
		t.Fatalf("location/event 未更新: %+v", updated)
	}
	if updated.CharExpressions[7] != "惊喜" {
		t.Fatalf("CharExpressions 未更新: %+v", updated.CharExpressions)
	}
	if len(updated.CharacterIDs) != 1 || updated.CharacterIDs[0] != 7 {
		t.Fatalf("CharacterIDs 未更新: %+v", updated)
	}
}

// TestSetImageAndStatus verifies SetPanelImage advances status to done and
// SetPanelStatus overwrites the status.
func TestSetImageAndStatus(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)
	out, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{{Caption: "p"}})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	id := out[0].ID

	img, err := env.panels.SetPanelImage(1, id, "http://img/x.png")
	if err != nil {
		t.Fatalf("set image: %v", err)
	}
	if img.ImageURL != "http://img/x.png" || img.Status != statusDone {
		t.Fatalf("set image should set url + status done: %+v", img)
	}

	st, err := env.panels.SetPanelStatus(1, id, "rendering")
	if err != nil {
		t.Fatalf("set status: %v", err)
	}
	if st.Status != "rendering" {
		t.Fatalf("status 未更新: %+v", st)
	}
}

// TestCrossUserIsolation is the most important test: panels under user 1's
// chapter must be invisible and inaccessible to user 2, and every cross-user
// operation must return ErrNotFound rather than leaking existence.
func TestCrossUserIsolation(t *testing.T) {
	env := newPanelTestEnv(t)
	ch := env.newChapter(t, 1)
	out, err := env.panels.ReplacePanels(1, ch.ID, []models.Panel{{Caption: "p"}})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	panelID := out[0].ID

	// User 2 replacing user 1's panels.
	if _, err := env.panels.ReplacePanels(2, ch.ID, []models.Panel{{Caption: "越权"}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户替换应返回 ErrNotFound, got %v", err)
	}
	// User 2 listing user 1's panels.
	if _, err := env.panels.ListPanels(2, ch.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户列出应返回 ErrNotFound, got %v", err)
	}
	// User 2 updating user 1's panel (resolves real chapter → blocked).
	if _, err := env.panels.UpdatePanel(2, panelID, models.Panel{Caption: "越权"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户更新应返回 ErrNotFound, got %v", err)
	}
	// User 2 setting image on user 1's panel.
	if _, err := env.panels.SetPanelImage(2, panelID, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户设图应返回 ErrNotFound, got %v", err)
	}
	// User 2 setting status on user 1's panel.
	if _, err := env.panels.SetPanelStatus(2, panelID, "rendering"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户改状态应返回 ErrNotFound, got %v", err)
	}

	// User 1's panel remains untouched.
	got, err := env.panels.ListPanels(1, ch.ID)
	if err != nil {
		t.Fatalf("list panels: %v", err)
	}
	if len(got) != 1 || got[0].Caption != "p" {
		t.Fatalf("越权操作后分镜不应改变: %+v", got)
	}
}

// TestUnknownPanelAndChapter verifies operations on nonexistent ids return
// ErrNotFound.
func TestUnknownPanelAndChapter(t *testing.T) {
	env := newPanelTestEnv(t)

	if _, err := env.panels.ListPanels(1, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知章节 List 应返回 ErrNotFound, got %v", err)
	}
	if _, err := env.panels.ReplacePanels(1, 999, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知章节 Replace 应返回 ErrNotFound, got %v", err)
	}
	if _, err := env.panels.UpdatePanel(1, 999, models.Panel{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知分镜 Update 应返回 ErrNotFound, got %v", err)
	}
}

// TestMarshalRoundTrip exercises the id serialization helpers directly,
// including the empty/whitespace guard.
func TestMarshalRoundTrip(t *testing.T) {
	s, err := marshalIDs(nil)
	if err != nil {
		t.Fatalf("marshal nil: %v", err)
	}
	if s != "[]" {
		t.Fatalf("nil 应序列化为 [], got %q", s)
	}

	for _, in := range []string{"", "   ", "[]"} {
		got, err := unmarshalIDs(in)
		if err != nil {
			t.Fatalf("unmarshal %q: %v", in, err)
		}
		if len(got) != 0 {
			t.Fatalf("unmarshal %q 应为空切片, got %+v", in, got)
		}
	}

	got, err := unmarshalIDs("[4,5,6]")
	if err != nil {
		t.Fatalf("unmarshal ids: %v", err)
	}
	if len(got) != 3 || got[0] != 4 || got[2] != 6 {
		t.Fatalf("往返错: %+v", got)
	}
}
