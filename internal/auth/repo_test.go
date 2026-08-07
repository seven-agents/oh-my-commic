package auth

import (
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestCreateAndFetch(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)

	u, err := repo.Create("小明", "hash")
	if err != nil || u.ID == 0 {
		t.Fatalf("create failed: %v", err)
	}
	if u.Nickname != "小明" || u.PasswordHash != "hash" || u.CreatedAt == "" {
		t.Fatalf("create returned unexpected user: %+v", u)
	}

	got, err := repo.ByNickname("小明")
	if err != nil || got.ID != u.ID {
		t.Fatalf("fetch mismatch: err=%v got=%+v want=%+v", err, got, u)
	}
	if got.PasswordHash != "hash" {
		t.Fatalf("fetched password_hash mismatch: %q", got.PasswordHash)
	}

	if _, err := repo.Create("小明", "hash2"); err == nil {
		t.Fatal("重复昵称应报错")
	}
}

func TestByNicknameNotFound(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	repo := NewUserRepo(d)

	if _, err := repo.ByNickname("不存在"); err == nil {
		t.Fatal("查询不存在的昵称应报错")
	}
}
