package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// tokenBytes is the number of random bytes backing each session token. 32 bytes
// (256 bits) is well beyond guessing range and encodes to a 64-char hex string.
const tokenBytes = 32

// Session is an in-memory session store mapping opaque tokens to user IDs.
//
// The store is safe for concurrent use by multiple HTTP handlers. It keeps no
// expiry of its own; sessions live until logout or process restart, which is
// acceptable for the current single-process deployment. A persistent or
// TTL-backed store can replace it later without changing callers.
type Session struct {
	mu     sync.RWMutex
	tokens map[string]int64
}

// NewSession returns an empty, ready-to-use session store.
func NewSession() *Session {
	return &Session{tokens: make(map[string]int64)}
}

// Issue creates a fresh cryptographically-random token bound to userID and
// records it. The generated token string is returned to hand back to the
// client (e.g. as a cookie value).
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
	s.mu.Unlock()

	return token
}

// UserID returns the user ID bound to token and whether the token is known.
func (s *Session) UserID(token string) (int64, bool) {
	s.mu.RLock()
	id, ok := s.tokens[token]
	s.mu.RUnlock()
	return id, ok
}

// Revoke removes token from the store. Revoking an unknown token is a no-op.
func (s *Session) Revoke(token string) {
	s.mu.Lock()
	delete(s.tokens, token)
	s.mu.Unlock()
}
