package community

import (
	"errors"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

func TestListPublicOrdersAndFiltersPrivate(t *testing.T) {
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if _, err := d.Exec(`INSERT INTO users (id, nickname, avatar_url, password_hash) VALUES (1,'小明','/media/users/1/a.png','h')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// 两本公开(不同 published_at) + 一本私密。
	if _, err := d.Exec(`INSERT INTO books (id,user_id,title,summary,is_public,published_at) VALUES
	  (10,1,'先发','s1',1,'2026-08-08 10:00:00'),
	  (11,1,'后发','s2',1,'2026-08-08 12:00:00'),
	  (12,1,'私密','s3',0,'')`); err != nil {
		t.Fatalf("seed books: %v", err)
	}

	repo := NewRepo(d)
	list, err := repo.ListPublic("", "new", 20, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 public, got %d", len(list))
	}
	if list[0].ID != 11 || list[1].ID != 10 {
		t.Fatalf("want newest-first [11,10], got [%d,%d]", list[0].ID, list[1].ID)
	}
	if list[0].Author.Nickname != "小明" || list[0].Author.AvatarURL != "/media/users/1/a.png" {
		t.Fatalf("author not populated: %+v", list[0].Author)
	}
	if list[0].Liked {
		t.Fatalf("anonymous liked must be false")
	}
}

func TestListPublicSortHot(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	// A 最新发布但 0 赞；B 较早发布但 5 赞。
	d.Exec(`INSERT INTO books (id,user_id,title,is_public,published_at,like_count) VALUES
	  (10,1,'A新但冷',1,'2026-08-08 12:00:00',0),
	  (11,1,'B旧但热',1,'2026-08-08 10:00:00',5)`)
	repo := NewRepo(d)

	// sort=new：按 published_at 降序 → A(10) 在前。
	byNew, err := repo.ListPublic("", "new", 20, 0)
	if err != nil || len(byNew) != 2 || byNew[0].ID != 10 {
		t.Fatalf("sort=new want [10,11], got %+v err=%v", ids(byNew), err)
	}
	// sort=hot：按 like_count 降序 → B(11) 在前。
	byHot, _ := repo.ListPublic("", "hot", 20, 0)
	if len(byHot) != 2 || byHot[0].ID != 11 {
		t.Fatalf("sort=hot want [11,10], got %+v", ids(byHot))
	}
	// 未知 sort 回落 new。
	byBad, _ := repo.ListPublic("", "'; DROP TABLE books;--", 20, 0)
	if len(byBad) != 2 || byBad[0].ID != 10 {
		t.Fatalf("unknown sort should fall back to new [10,11], got %+v", ids(byBad))
	}
}

func ids(bs []CommunityBook) []int64 {
	out := make([]int64, len(bs))
	for i, b := range bs {
		out[i] = b.ID
	}
	return out
}

func TestGetPublicDetailPrivateIs404(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,is_public) VALUES (12,1,'私密',0)`)

	repo := NewRepo(d)
	if _, err := repo.GetPublicDetail("", 12); !errors.Is(err, ErrNotFound) {
		t.Fatalf("private book detail must be ErrNotFound, got %v", err)
	}
	if _, err := repo.GetPublicDetail("", 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing book detail must be ErrNotFound, got %v", err)
	}
}

func TestGetPublicDetailReturnsChaptersWithPanels(t *testing.T) {
	d, _ := db.Open(":memory:")
	t.Cleanup(func() { d.Close() })
	d.Exec(`INSERT INTO users (id, nickname, password_hash) VALUES (1,'小明','h')`)
	d.Exec(`INSERT INTO books (id,user_id,title,summary,style,is_public,published_at) VALUES (10,1,'书','梗概','ghibli',1,'2026-08-08 10:00:00')`)
	d.Exec(`INSERT INTO chapters (id,book_id,"order",title,summary,is_cover) VALUES (100,10,0,'封面','',1),(101,10,1,'第一章','章梗概',0)`)
	d.Exec(`INSERT INTO panels (id,chapter_id,"order",caption,image_url,status) VALUES (1000,101,0,'旁白','/media/10/x.png','done')`)

	repo := NewRepo(d)
	detail, err := repo.GetPublicDetail("", 10)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Title != "书" || detail.Author.Nickname != "小明" {
		t.Fatalf("detail meta wrong: %+v", detail)
	}
	if len(detail.Chapters) != 2 {
		t.Fatalf("want 2 chapters, got %d", len(detail.Chapters))
	}
	var first *ReaderChapter
	for i := range detail.Chapters {
		if detail.Chapters[i].Order == 1 {
			first = &detail.Chapters[i]
		}
	}
	if first == nil || len(first.Panels) != 1 || first.Panels[0].ImageURL != "/media/10/x.png" {
		t.Fatalf("chapter panels not populated: %+v", detail.Chapters)
	}
}
