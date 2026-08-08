package community

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestLikeIsIdempotentAndMaintainsCount(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h'),(2,'小红','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'书',1,'t')`)
	repo := NewRepo(d)

	res, err := repo.Like(2, 10)
	if err != nil || res.LikeCount != 1 || !res.Liked {
		t.Fatalf("first like: res=%+v err=%v", res, err)
	}
	// 重复赞：仍为 1（幂等）。
	res, err = repo.Like(2, 10)
	if err != nil || res.LikeCount != 1 {
		t.Fatalf("dup like should stay 1: res=%+v err=%v", res, err)
	}
	// 另一个用户赞：2。
	res, _ = repo.Like(1, 10)
	if res.LikeCount != 2 {
		t.Fatalf("second user like should be 2, got %d", res.LikeCount)
	}
	// 取消赞：回到 1；再取消不为负。
	res, err = repo.Unlike(2, 10)
	if err != nil || res.LikeCount != 1 || res.Liked {
		t.Fatalf("unlike: res=%+v err=%v", res, err)
	}
	res, _ = repo.Unlike(2, 10)
	if res.LikeCount != 1 {
		t.Fatalf("dup unlike should stay 1, got %d", res.LikeCount)
	}
}

func TestLikePrivateBookIs404(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, password_hash) VALUES (1,'h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public) VALUES (12,1,'私密',0)`)
	repo := NewRepo(d)
	if _, err := repo.Like(1, 12); !errors.Is(err, ErrNotFound) {
		t.Fatalf("like private must be ErrNotFound, got %v", err)
	}
}

func TestRecordViewDedupes(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, password_hash) VALUES (1,'h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at) VALUES (10,1,'书',1,'t')`)
	repo := NewRepo(d)

	if err := repo.RecordView(10, "anon:abc"); err != nil {
		t.Fatalf("view1: %v", err)
	}
	if err := repo.RecordView(10, "anon:abc"); err != nil { // 同 key 去重
		t.Fatalf("view dup: %v", err)
	}
	if err := repo.RecordView(10, "u:1"); err != nil { // 不同 key +1
		t.Fatalf("view2: %v", err)
	}
	var vc int
	d.QueryRow(`SELECT view_count FROM books WHERE id=10`).Scan(&vc)
	if vc != 2 {
		t.Fatalf("unique viewers should be 2, got %d", vc)
	}
	// 非公开：404。
	d.Exec(`INSERT INTO books (id,user_id,title,is_public) VALUES (12,1,'私',0)`)
	if err := repo.RecordView(12, "anon:x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("view private must be ErrNotFound, got %v", err)
	}
}
