package asset

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/book"
	"github.com/seven-agents/oh-my-commic/internal/db"
	"github.com/seven-agents/oh-my-commic/internal/models"
)

// assetTestEnv bundles the collaborators a Service needs: a book.Repo (the
// ownership gate), the asset.Repo, and the asset.Service under test, all sharing
// one in-memory DB.
type assetTestEnv struct {
	repo   *Repo
	books  *book.Repo
	assets *Service
}

// newAssetTestEnv opens an in-memory database, seeds two users (ids 1 and 2 —
// required because books.user_id has a foreign key to users), and wires a
// book.Repo + asset.Repo + asset.Service on top of it. The DB is closed
// automatically when the test finishes.
func newAssetTestEnv(t *testing.T) *assetTestEnv {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	seedUsers(t, d, 2)

	books := book.NewRepo(d)
	repo := NewRepo(d)
	assets := NewService(repo, books)
	return &assetTestEnv{repo: repo, books: books, assets: assets}
}

// seedUsers inserts n users so that books referencing user ids 1..n satisfy the
// foreign key constraint.
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

// TestCharacterBelongsToOwnedBook is the most important test: a character may
// only be created under a book the caller owns. User 2 must not be able to add
// a character to user 1's book, and the failure must be ErrNotFound so the
// existence of the book never leaks.
func TestCharacterBelongsToOwnedBook(t *testing.T) {
	env := newAssetTestEnv(t)
	b, err := env.books.Create(1, "书", "ghibli", "")
	if err != nil {
		t.Fatalf("create book: %v", err)
	}

	// 用户 2 在用户 1 的书下建角色 → 失败(not found)。
	if _, err := env.assets.CreateCharacter(2, b.ID, models.Character{Name: "狐狸", Type: "character"}); err == nil {
		t.Fatal("越权建角色应失败")
	} else if !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权建角色应返回 ErrNotFound, got %v", err)
	}

	// 本人建角色成功。
	c, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "狐狸", Type: "character"})
	if err != nil || c.ID == 0 {
		t.Fatalf("本人建角色应成功: %v", err)
	}
	if c.BookID != b.ID {
		t.Fatalf("角色应归属书 %d, got %d", b.ID, c.BookID)
	}
}

// TestCreateCharacterDefaultsType verifies an empty Type falls back to
// "character".
func TestCreateCharacterDefaultsType(t *testing.T) {
	env := newAssetTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	c, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "无类型"})
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	if c.Type != "character" {
		t.Fatalf("空 type 应默认为 character, got %q", c.Type)
	}
}

// TestListCharactersIsolation proves user 2 cannot list characters under user
// 1's book, while the owner sees them.
func TestListCharactersIsolation(t *testing.T) {
	env := newAssetTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	if _, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "狐狸"}); err != nil {
		t.Fatalf("seed character: %v", err)
	}

	if _, err := env.assets.ListCharacters(2, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权列角色应返回 ErrNotFound, got %v", err)
	}

	list, err := env.assets.ListCharacters(1, b.ID)
	if err != nil {
		t.Fatalf("本人列角色应成功: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("期望 1 个角色, got %d", len(list))
	}
}

// TestCrossUserUpdateDeleteCharacterBlocked proves that user 2 can neither
// update nor delete a character that lives under user 1's book, and that the
// asset is left unchanged.
func TestCrossUserUpdateDeleteCharacterBlocked(t *testing.T) {
	env := newAssetTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	c, err := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "狐狸", Description: "原始"})
	if err != nil {
		t.Fatalf("seed character: %v", err)
	}

	// 越权更新失败。
	if _, err := env.assets.UpdateCharacter(2, c.ID, models.Character{Name: "篡改", Description: "改动"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权更新应返回 ErrNotFound, got %v", err)
	}
	// 越权删除失败。
	if err := env.assets.DeleteCharacter(2, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权删除应返回 ErrNotFound, got %v", err)
	}

	// 资产保持不变。
	list, err := env.assets.ListCharacters(1, b.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Description != "原始" {
		t.Fatalf("角色应保持不变, got %+v", list)
	}
}

// TestOwnerUpdateDeleteCharacter proves the owner can update and delete.
func TestOwnerUpdateDeleteCharacter(t *testing.T) {
	env := newAssetTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	c, _ := env.assets.CreateCharacter(1, b.ID, models.Character{Name: "狐狸", Description: "原始"})

	updated, err := env.assets.UpdateCharacter(1, c.ID, models.Character{Name: "狐狸2", Description: "新的", Type: "character"})
	if err != nil {
		t.Fatalf("owner update: %v", err)
	}
	if updated.Description != "新的" || updated.BookID != b.ID {
		t.Fatalf("更新结果异常: %+v", updated)
	}

	if err := env.assets.DeleteCharacter(1, c.ID); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
	list, _ := env.assets.ListCharacters(1, b.ID)
	if len(list) != 0 {
		t.Fatalf("删除后应为空, got %d", len(list))
	}
}

// TestSceneBelongsToOwnedBook is the scene-side symmetric ownership test.
func TestSceneBelongsToOwnedBook(t *testing.T) {
	env := newAssetTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")

	if _, err := env.assets.CreateScene(2, b.ID, models.Scene{Name: "森林"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权建场景应返回 ErrNotFound, got %v", err)
	}

	s, err := env.assets.CreateScene(1, b.ID, models.Scene{Name: "森林"})
	if err != nil || s.ID == 0 {
		t.Fatalf("本人建场景应成功: %v", err)
	}
	if s.BookID != b.ID {
		t.Fatalf("场景应归属书 %d, got %d", b.ID, s.BookID)
	}
}

// TestSceneListAndCrossUserBlocked mirrors the character isolation checks for
// scenes: list isolation plus blocked cross-user update/delete.
func TestSceneListAndCrossUserBlocked(t *testing.T) {
	env := newAssetTestEnv(t)
	b, _ := env.books.Create(1, "书", "ghibli", "")
	s, err := env.assets.CreateScene(1, b.ID, models.Scene{Name: "森林", Description: "原始"})
	if err != nil {
		t.Fatalf("seed scene: %v", err)
	}

	if _, err := env.assets.ListScenes(2, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权列场景应返回 ErrNotFound, got %v", err)
	}
	if _, err := env.assets.UpdateScene(2, s.ID, models.Scene{Name: "篡改"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权更新场景应返回 ErrNotFound, got %v", err)
	}
	if err := env.assets.DeleteScene(2, s.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("越权删除场景应返回 ErrNotFound, got %v", err)
	}

	list, err := env.assets.ListScenes(1, b.ID)
	if err != nil {
		t.Fatalf("list scenes: %v", err)
	}
	if len(list) != 1 || list[0].Description != "原始" {
		t.Fatalf("场景应保持不变, got %+v", list)
	}

	updated, err := env.assets.UpdateScene(1, s.ID, models.Scene{Name: "森林2", Description: "新的"})
	if err != nil {
		t.Fatalf("owner update scene: %v", err)
	}
	if updated.Description != "新的" {
		t.Fatalf("场景更新结果异常: %+v", updated)
	}
	if err := env.assets.DeleteScene(1, s.ID); err != nil {
		t.Fatalf("owner delete scene: %v", err)
	}
}

// TestRepoNotFound exercises the repo directly for missing ids across get,
// update, and delete on both characters and scenes.
func TestRepoNotFound(t *testing.T) {
	env := newAssetTestEnv(t)
	r := env.repo

	if _, err := r.GetCharacter(9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失角色 Get 应返回 ErrNotFound, got %v", err)
	}
	if _, err := r.UpdateCharacter(9999, models.Character{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失角色 Update 应返回 ErrNotFound, got %v", err)
	}
	if err := r.DeleteCharacter(9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失角色 Delete 应返回 ErrNotFound, got %v", err)
	}

	if _, err := r.GetScene(9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失场景 Get 应返回 ErrNotFound, got %v", err)
	}
	if _, err := r.UpdateScene(9999, models.Scene{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失场景 Update 应返回 ErrNotFound, got %v", err)
	}
	if err := r.DeleteScene(9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失场景 Delete 应返回 ErrNotFound, got %v", err)
	}
}

// TestServiceMissingAssetNotFound proves the Service surfaces ErrNotFound when
// the targeted character or scene does not exist at all (repo GetCharacter /
// GetScene miss).
func TestServiceMissingAssetNotFound(t *testing.T) {
	env := newAssetTestEnv(t)

	if _, err := env.assets.UpdateCharacter(1, 9999, models.Character{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("更新缺失角色应返回 ErrNotFound, got %v", err)
	}
	if err := env.assets.DeleteCharacter(1, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除缺失角色应返回 ErrNotFound, got %v", err)
	}
	if _, err := env.assets.UpdateScene(1, 9999, models.Scene{Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("更新缺失场景应返回 ErrNotFound, got %v", err)
	}
	if err := env.assets.DeleteScene(1, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除缺失场景应返回 ErrNotFound, got %v", err)
	}
}
