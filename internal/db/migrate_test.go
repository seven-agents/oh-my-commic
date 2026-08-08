package db

import "testing"

func TestMigrateAddsCommunityTablesAndColumns(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	// books 新列可写可读，默认 0/''。
	res, err := d.Exec(`INSERT INTO users (password_hash) VALUES ('h')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	uid, _ := res.LastInsertId()
	res, err = d.Exec(`INSERT INTO books (user_id, title) VALUES (?, '书')`, uid)
	if err != nil {
		t.Fatalf("insert book: %v", err)
	}
	bid, _ := res.LastInsertId()

	var like, view int
	var published string
	if err := d.QueryRow(
		`SELECT like_count, view_count, published_at FROM books WHERE id = ?`, bid,
	).Scan(&like, &view, &published); err != nil {
		t.Fatalf("select new columns: %v", err)
	}
	if like != 0 || view != 0 || published != "" {
		t.Fatalf("defaults wrong: like=%d view=%d published=%q", like, view, published)
	}

	// 两张新表存在且复合主键去重。
	if _, err := d.Exec(`INSERT INTO book_likes (book_id, user_id) VALUES (?, ?)`, bid, uid); err != nil {
		t.Fatalf("insert like: %v", err)
	}
	if _, err := d.Exec(`INSERT OR IGNORE INTO book_likes (book_id, user_id) VALUES (?, ?)`, bid, uid); err != nil {
		t.Fatalf("dup like should be ignored: %v", err)
	}
	if _, err := d.Exec(`INSERT INTO book_views (book_id, viewer_key) VALUES (?, 'anon:x')`, bid); err != nil {
		t.Fatalf("insert view: %v", err)
	}

	// 幂等：再次 Migrate 不报错。
	if err := Migrate(d); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
}
