package auth

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
)

// inviteKey is the settings row key under which the single global invite code
// is stored. Registration is gated on a visitor supplying this code.
const inviteKey = "invite_code"

// codeAlphabet is the character set for generated invite codes: lowercase
// letters and digits, chosen for easy transcription (no case ambiguity).
const codeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// codeLength is the number of characters in a generated invite code.
const codeLength = 10

// InviteRepo reads and writes the single global invite code, persisted as one
// row in the settings key/value table.
type InviteRepo struct {
	db *sql.DB
}

// NewInviteRepo returns an InviteRepo backed by the given database handle.
func NewInviteRepo(d *sql.DB) *InviteRepo {
	return &InviteRepo{db: d}
}

// Get returns the current invite code, or "" if none has been set yet. A missing
// row is not an error — it is the initial state before Seed/Set.
func (r *InviteRepo) Get() (string, error) {
	var value string
	err := r.db.QueryRow(
		`SELECT value FROM settings WHERE key = ?`,
		inviteKey,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get invite code: %w", err)
	}
	return value, nil
}

// Set upserts the invite code, replacing any existing value.
func (r *InviteRepo) Set(code string) error {
	if _, err := r.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		inviteKey, code,
	); err != nil {
		return fmt.Errorf("set invite code: %w", err)
	}
	return nil
}

// Rotate generates a fresh random code, persists it, and returns it. Any
// previous code is overwritten.
func (r *InviteRepo) Rotate() (string, error) {
	code := randomCode()
	if err := r.Set(code); err != nil {
		return "", fmt.Errorf("rotate invite code: %w", err)
	}
	return code, nil
}

// Seed ensures an invite code exists and returns the effective value. If one is
// already set, it is returned unchanged (idempotent). Otherwise the preferred
// value is used when non-empty, else a fresh random code is generated; the
// chosen value is persisted and returned.
func (r *InviteRepo) Seed(preferred string) (string, error) {
	existing, err := r.Get()
	if err != nil {
		return "", fmt.Errorf("seed invite code: %w", err)
	}
	if existing != "" {
		return existing, nil
	}
	code := preferred
	if code == "" {
		code = randomCode()
	}
	if err := r.Set(code); err != nil {
		return "", fmt.Errorf("seed invite code: %w", err)
	}
	return code, nil
}

// randomCode returns a codeLength-character code drawn uniformly from
// codeAlphabet using crypto/rand. The alphabet length (36) evenly divides 252
// (the largest multiple of 36 below 256); bytes >= 252 are rejected to avoid
// modulo bias, so every character is uniformly distributed.
func randomCode() string {
	const max = 256 - (256 % len(codeAlphabet)) // 252: rejection threshold
	out := make([]byte, codeLength)
	buf := make([]byte, 1)
	for i := 0; i < codeLength; {
		if _, err := rand.Read(buf); err != nil {
			// crypto/rand.Read never returns an error on supported platforms;
			// panicking here surfaces the impossible case rather than silently
			// emitting a weak code.
			panic(fmt.Sprintf("crypto/rand: %v", err))
		}
		if int(buf[0]) >= max {
			continue // reject to keep the distribution uniform
		}
		out[i] = codeAlphabet[int(buf[0])%len(codeAlphabet)]
		i++
	}
	return string(out)
}
