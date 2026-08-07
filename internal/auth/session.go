package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"sync"
)

// tokenBytes is the number of random bytes backing each session token. 32 bytes
// (256 bits) is well beyond guessing range and encodes to a 64-char hex string.
const tokenBytes = 32

// Session is a session store mapping opaque tokens to user IDs.
//
// The in-memory map is authoritative for reads and is safe for concurrent use
// by multiple HTTP handlers. When constructed with a non-nil *sql.DB the store
// is DB-backed: existing rows are loaded on construction (so live sessions
// survive a process restart) and every Issue/Revoke is written through to the
// sessions table. Constructed with a nil db it behaves as a pure in-memory
// store, which is what unit tests use.
//
// DB writes are fail-soft: an error persisting a token never breaks auth for
// the current process; it is logged and the in-memory map remains the source of
// truth. The store keeps no expiry of its own; sessions live until logout.
type Session struct {
	mu     sync.RWMutex
	tokens map[string]int64
	db     *sql.DB
}

// NewSession returns a ready-to-use session store.
//
// When db is non-nil, all existing (token, user_id) rows are loaded into the
// in-memory map so sessions issued before a restart remain valid. When db is
// nil the store is purely in-memory.
func NewSession(db *sql.DB) *Session {
	s := &Session{
		tokens: make(map[string]int64),
		db:     db,
	}
	if db != nil {
		s.load()
	}
	return s
}

// load repopulates the in-memory map from the sessions table. Errors are logged
// and otherwise tolerated: a store that cannot read past sessions still works
// for new logins.
func (s *Session) load() {
	rows, err := s.db.Query("SELECT token, user_id FROM sessions")
	if err != nil {
		log.Printf("auth: load sessions: %v", err)
		return
	}
	defer rows.Close()

	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var (
			token  string
			userID int64
		)
		if err := rows.Scan(&token, &userID); err != nil {
			log.Printf("auth: scan session row: %v", err)
			continue
		}
		s.tokens[token] = userID
	}
	if err := rows.Err(); err != nil {
		log.Printf("auth: iterate session rows: %v", err)
	}
}

// Issue creates a fresh cryptographically-random token bound to userID and
// records it. The generated token string is returned to hand back to the
// client (e.g. as a cookie value).
//
// When DB-backed, the token is written through to the sessions table. A DB
// write error is logged but never fatal: the token is still added to the
// in-memory map so auth keeps working for the process lifetime.
//
// It panics only if the system CSPRNG fails, which indicates a broken host and
// is not a recoverable condition for an auth system.
func (s *Session) Issue(userID int64) string {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	token := hex.EncodeToString(buf)

	s.mu.Lock()
	s.tokens[token] = userID
	db := s.db
	s.mu.Unlock()

	if db != nil {
		if _, err := db.Exec(
			"INSERT INTO sessions (token, user_id) VALUES (?, ?)", token, userID,
		); err != nil {
			log.Printf("auth: persist session: %v", err)
		}
	}

	return token
}

// UserID returns the user ID bound to token and whether the token is known.
// It reads the in-memory map, which is authoritative after load + write-through.
func (s *Session) UserID(token string) (int64, bool) {
	s.mu.RLock()
	id, ok := s.tokens[token]
	s.mu.RUnlock()
	return id, ok
}

// Revoke removes token from the store. Revoking an unknown token is a no-op.
// When DB-backed, the row is also deleted; a delete error is logged but not
// fatal so logout always clears the in-memory session.
func (s *Session) Revoke(token string) {
	s.mu.Lock()
	delete(s.tokens, token)
	db := s.db
	s.mu.Unlock()

	if db != nil {
		if _, err := db.Exec("DELETE FROM sessions WHERE token=?", token); err != nil {
			log.Printf("auth: revoke session: %v", err)
		}
	}
}
