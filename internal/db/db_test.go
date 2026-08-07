package db

import (
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
)

func TestMigrateCreatesTables(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, tbl := range []string{"users", "books", "characters", "scenes", "chapters", "panels", "sessions"} {
		var name string
		err := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Fatalf("表 %s 不存在: %v", tbl, err)
		}
	}
}

// TestPanelsStructuredColumns verifies the structured storyboard columns exist on
// the panels table for a fresh DB (created inline by the CREATE TABLE statement).
func TestPanelsStructuredColumns(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	cols := panelColumnSet(t, d)
	for _, want := range []string{"location", "event", "char_expressions"} {
		if !cols[want] {
			t.Fatalf("panels 表缺少列 %q，实际列: %v", want, cols)
		}
	}
}

// TestMigrateAddsColumnsToLegacyDB proves the idempotent ALTER path: a legacy
// panels table without the structured columns gains them after Migrate, and a
// second Migrate run does not fail on the duplicate columns.
func TestMigrateAddsColumnsToLegacyDB(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Drop and recreate panels WITHOUT the new columns to simulate a legacy DB.
	if _, err := d.Exec(`DROP TABLE panels`); err != nil {
		t.Fatalf("drop panels: %v", err)
	}
	if _, err := d.Exec(`CREATE TABLE panels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  chapter_id INTEGER NOT NULL,
  "order" INTEGER NOT NULL DEFAULT 0,
  caption TEXT NOT NULL DEFAULT '',
  character_ids TEXT NOT NULL DEFAULT '[]',
  scene_id INTEGER NOT NULL DEFAULT 0,
  image_prompt TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending'
)`); err != nil {
		t.Fatalf("create legacy panels: %v", err)
	}

	if err := Migrate(d); err != nil {
		t.Fatalf("migrate legacy DB: %v", err)
	}
	cols := panelColumnSet(t, d)
	for _, want := range []string{"location", "event", "char_expressions"} {
		if !cols[want] {
			t.Fatalf("迁移后 panels 表仍缺列 %q，实际列: %v", want, cols)
		}
	}

	// A repeat Migrate must tolerate the now-duplicate columns.
	if err := Migrate(d); err != nil {
		t.Fatalf("重复迁移应幂等，got: %v", err)
	}
}

// panelColumnSet returns the set of column names on the panels table.
func panelColumnSet(t *testing.T, d *sql.DB) map[string]bool {
	t.Helper()
	rows, err := d.Query(`PRAGMA table_info(panels)`)
	if err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			dfltValue  sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &primaryKey); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return cols
}

// TestForeignKeyCascadeDelete proves that ON DELETE CASCADE fires, which only
// works when foreign_keys is enabled on the connection that runs the DELETE.
// With the old single d.Exec("PRAGMA foreign_keys=ON") the pragma was not
// reliably applied across the pool and the child row would survive; with the
// DSN-based pragma it is enforced on every connection.
func TestForeignKeyCascadeDelete(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	res, err := d.Exec(
		"INSERT INTO users (nickname, password_hash) VALUES (?, ?)",
		"alice", "hash",
	)
	if err != nil {
		t.Fatalf("插入 user 失败: %v", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("获取 user id 失败: %v", err)
	}

	if _, err := d.Exec(
		"INSERT INTO books (user_id, title) VALUES (?, ?)",
		userID, "my book",
	); err != nil {
		t.Fatalf("插入 book 失败: %v", err)
	}

	if _, err := d.Exec("DELETE FROM users WHERE id=?", userID); err != nil {
		t.Fatalf("删除 user 失败: %v", err)
	}

	var count int
	if err := d.QueryRow(
		"SELECT COUNT(*) FROM books WHERE user_id=?", userID,
	).Scan(&count); err != nil {
		t.Fatalf("统计 books 失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("级联删除未生效: 期望 book 行为 0，实际为 %d", count)
	}
}

// TestForeignKeyCascadePooledConnections proves the pragma is applied to EVERY
// pooled connection, not just one. It uses a file-backed DB (so the pool may
// open several physical connections) and forces concurrency to spread work
// across them. With the old single d.Exec("PRAGMA foreign_keys=ON") only one
// arbitrary connection had FKs enabled, so a DELETE served by another
// connection would leave the child row behind. With the DSN pragma every
// connection enforces the cascade, so no orphan rows survive.
func TestForeignKeyCascadePooledConnections(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cascade.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	// Force the pool to open multiple physical connections concurrently so the
	// per-connection pragma is exercised on more than one of them.
	d.SetMaxOpenConns(8)

	const n = 32
	ids := make([]int64, n)
	var wg sync.WaitGroup
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := d.Exec(
				"INSERT INTO users (nickname, password_hash) VALUES (?, ?)",
				nickname(i), "hash",
			)
			if err != nil {
				errCh <- err
				return
			}
			uid, err := res.LastInsertId()
			if err != nil {
				errCh <- err
				return
			}
			ids[i] = uid
			if _, err := d.Exec(
				"INSERT INTO books (user_id, title) VALUES (?, ?)", uid, "b",
			); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("并发插入失败: %v", err)
		}
	}

	// Delete all users concurrently; each DELETE may land on a different pooled
	// connection. Every one must cascade.
	wg = sync.WaitGroup{}
	delErr := make(chan error, n)
	for _, id := range ids {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			if _, err := d.Exec("DELETE FROM users WHERE id=?", id); err != nil {
				delErr <- err
			}
		}(id)
	}
	wg.Wait()
	close(delErr)
	for err := range delErr {
		if err != nil {
			t.Fatalf("并发删除失败: %v", err)
		}
	}

	var remaining int
	if err := d.QueryRow("SELECT COUNT(*) FROM books").Scan(&remaining); err != nil {
		t.Fatalf("统计 books 失败: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("池级联删除未生效: 仍有 %d 个孤立 book 行 (说明某些连接未启用 foreign_keys)", remaining)
	}
}

func nickname(i int) string {
	return "user-" + string(rune('a'+i%26)) + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
