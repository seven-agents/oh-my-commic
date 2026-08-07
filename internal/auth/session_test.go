package auth

import (
	"database/sql"
	"testing"

	"github.com/seven-agents/oh-my-commic/internal/db"
)

// openSeededDB opens an in-memory database and inserts one user, returning the
// db and that user's id. In-memory databases are pinned to a single connection
// by db.Open, so the schema and rows persist across the multiple NewSession
// calls a test uses to simulate a restart.
func openSeededDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()

	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	res, err := d.Exec(
		"INSERT INTO users (nickname, password_hash) VALUES (?, ?)",
		"alice", "hash",
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("user id: %v", err)
	}
	return d, userID
}

// TestSessionPersistsAcrossRestart proves that a token issued by one Session is
// still valid after a fresh Session loads from the same database, simulating a
// process restart.
func TestSessionPersistsAcrossRestart(t *testing.T) {
	d, userID := openSeededDB(t)

	s1 := NewSession(d)
	tok := s1.Issue(userID)

	// A fresh store re-loads live sessions from the same db (the "restart").
	s2 := NewSession(d)
	got, ok := s2.UserID(tok)
	if !ok {
		t.Fatal("token 未在重启后恢复")
	}
	if got != userID {
		t.Fatalf("期望 userID=%d, 实际=%d", userID, got)
	}
}

// TestSessionRevokePersists proves a revoked token stays revoked after restart.
func TestSessionRevokePersists(t *testing.T) {
	d, userID := openSeededDB(t)

	s1 := NewSession(d)
	tok := s1.Issue(userID)
	s1.Revoke(tok)

	s3 := NewSession(d)
	if _, ok := s3.UserID(tok); ok {
		t.Fatal("已撤销的 token 不应在重启后仍有效")
	}
}

// TestSessionNilDB proves the store behaves as a pure in-memory store when
// constructed without a database.
func TestSessionNilDB(t *testing.T) {
	s := NewSession(nil)

	tok := s.Issue(7)
	got, ok := s.UserID(tok)
	if !ok || got != 7 {
		t.Fatalf("nil-db Issue/UserID 失败: got=%d ok=%v", got, ok)
	}

	s.Revoke(tok)
	if _, ok := s.UserID(tok); ok {
		t.Fatal("nil-db Revoke 未移除 token")
	}

	if _, ok := s.UserID("unknown"); ok {
		t.Fatal("未知 token 不应有效")
	}
}
