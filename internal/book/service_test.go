package book

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

// newTestBookRepo opens an in-memory database, seeds two users (ids 1 and 2 —
// required because books.user_id has a foreign key to users), and returns a Repo
// bound to it. The DB is closed automatically when the test finishes.
func newTestBookRepo(t *testing.T) *Repo {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	seedUsers(t, d, 2)
	return NewRepo(d)
}

// seedUsers inserts n users so that books referencing user ids 1..n satisfy the
// foreign key constraint.
func seedUsers(t *testing.T, d *sql.DB, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		if _, err := d.Exec(
			`INSERT INTO users (nickname, password_hash) VALUES (?, ?)`,
			nickname(i), "hash",
		); err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}
}

func nickname(i int) string {
	return "user" + string(rune('0'+i))
}

// TestBookIsolation is the most important test: a book created by user 1 must be
// invisible and inaccessible to user 2, and cross-user access must return an
// error (not found) rather than leaking existence.
func TestBookIsolation(t *testing.T) {
	repo := newTestBookRepo(t)
	b, err := repo.Create(1, "A的书", "ghibli", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 用户 2 不能拿到用户 1 的书。
	if _, err := repo.Get(2, b.ID); err == nil {
		t.Fatal("越权访问应失败(not found)")
	} else if !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权访问应返回 ErrNotFound, got %v", err)
	}

	// 本人可以。
	if _, err := repo.Get(1, b.ID); err != nil {
		t.Fatalf("本人应可访问: %v", err)
	}

	// 列表隔离。
	list, err := repo.ListByUser(2)
	if err != nil {
		t.Fatalf("list user 2: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("用户2 不应看到任何书, got %d", len(list))
	}
}

func TestCreateDefaultsStyle(t *testing.T) {
	repo := newTestBookRepo(t)

	b, err := repo.Create(1, "无风格", "", "简介")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.Style != "ghibli" {
		t.Fatalf("style 应默认 ghibli, got %q", b.Style)
	}
	if b.ID == 0 || b.UserID != 1 || b.Title != "无风格" || b.Summary != "简介" {
		t.Fatalf("create returned unexpected book: %+v", b)
	}
	if b.CreatedAt == "" || b.UpdatedAt == "" {
		t.Fatalf("timestamps not populated: %+v", b)
	}
}

func TestListByUserOrdersDesc(t *testing.T) {
	repo := newTestBookRepo(t)

	first, _ := repo.Create(1, "第一本", "ghibli", "")
	second, _ := repo.Create(1, "第二本", "ghibli", "")

	list, err := repo.ListByUser(1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 books, got %d", len(list))
	}
	// id desc: most recently created first.
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Fatalf("expected id-desc order, got %d then %d", list[0].ID, list[1].ID)
	}
}

func TestUpdateRoundTrip(t *testing.T) {
	repo := newTestBookRepo(t)
	b, _ := repo.Create(1, "旧标题", "ghibli", "旧简介")

	updated, err := repo.Update(1, b.ID, "新标题", "manga", "新简介")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "新标题" || updated.Style != "manga" || updated.Summary != "新简介" {
		t.Fatalf("update did not persist: %+v", updated)
	}

	got, _ := repo.Get(1, b.ID)
	if got.Title != "新标题" || got.Style != "manga" {
		t.Fatalf("update not visible on re-read: %+v", got)
	}
}

func TestUpdateCrossUserBlocked(t *testing.T) {
	repo := newTestBookRepo(t)
	b, _ := repo.Create(1, "标题", "ghibli", "")

	if _, err := repo.Update(2, b.ID, "篡改", "manga", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户更新应返回 ErrNotFound, got %v", err)
	}

	// Original must be untouched.
	got, _ := repo.Get(1, b.ID)
	if got.Title != "标题" {
		t.Fatalf("跨用户更新篡改了原书: %+v", got)
	}
}

func TestDeleteRoundTrip(t *testing.T) {
	repo := newTestBookRepo(t)
	b, _ := repo.Create(1, "待删", "ghibli", "")

	if err := repo.Delete(1, b.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(1, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后应查不到, got %v", err)
	}
	// Deleting again returns ErrNotFound.
	if err := repo.Delete(1, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除应返回 ErrNotFound, got %v", err)
	}
}

func TestDeleteCrossUserBlocked(t *testing.T) {
	repo := newTestBookRepo(t)
	b, _ := repo.Create(1, "标题", "ghibli", "")

	if err := repo.Delete(2, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("跨用户删除应返回 ErrNotFound, got %v", err)
	}
	if _, err := repo.Get(1, b.ID); err != nil {
		t.Fatalf("跨用户删除误删了原书: %v", err)
	}
}

func TestServiceCreateValidatesTitle(t *testing.T) {
	svc := NewService(newTestBookRepo(t))

	if _, err := svc.Create(1, "   ", "ghibli", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("空标题应返回 ErrValidation, got %v", err)
	}

	b, err := svc.Create(1, "  有效标题  ", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.Title != "有效标题" {
		t.Fatalf("service 应 trim 标题, got %q", b.Title)
	}
}

func TestServiceUpdateValidatesTitle(t *testing.T) {
	svc := NewService(newTestBookRepo(t))
	b, _ := svc.Create(1, "标题", "ghibli", "")

	if _, err := svc.Update(1, b.ID, "", "manga", ""); !errors.Is(err, ErrValidation) {
		t.Fatalf("空标题更新应返回 ErrValidation, got %v", err)
	}
}

func TestSetVisibilityPublishesAndUnpublishes(t *testing.T) {
	svc := NewService(newTestBookRepo(t))
	b, err := svc.Create(1, "书", "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 发布：is_public=true 且 published_at 非空。
	pub, err := svc.SetVisibility(1, b.ID, true)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !pub.IsPublic || pub.PublishedAt == "" {
		t.Fatalf("publish should set is_public + published_at: %+v", pub)
	}

	// 下架：is_public=false，published_at 保留旧值。
	un, err := svc.SetVisibility(1, b.ID, false)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if un.IsPublic {
		t.Fatalf("unpublish should clear is_public: %+v", un)
	}
	if un.PublishedAt != pub.PublishedAt {
		t.Fatalf("unpublish must keep published_at: got %q want %q", un.PublishedAt, pub.PublishedAt)
	}

	// 非 owner：404。
	if _, err := svc.SetVisibility(2, b.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner should get ErrNotFound, got %v", err)
	}
}
