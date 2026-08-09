package auth

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// inviteKey is the settings row key under which the single global invite code
// is stored. Registration is gated on a visitor supplying this code.
const inviteKey = "invite_code"

// inviteUsedKey is the settings row key holding how many successful
// registrations the current invite code has consumed. It resets to 0 whenever
// the code is rotated. A missing row is treated as 0 (the pre-limit state), so
// existing deployments start counting from zero without a data migration.
const inviteUsedKey = "invite_used"

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
// previous code is overwritten and the usage counter is reset to 0 (a new code
// grants a fresh allowance) — both in one transaction so the code and its
// counter never disagree.
func (r *InviteRepo) Rotate() (string, error) {
	code := randomCode()

	tx, err := r.db.Begin()
	if err != nil {
		return "", fmt.Errorf("rotate invite code: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		inviteKey, code,
	); err != nil {
		return "", fmt.Errorf("rotate invite code: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO settings(key, value) VALUES(?, '0')
		 ON CONFLICT(key) DO UPDATE SET value = '0'`,
		inviteUsedKey,
	); err != nil {
		return "", fmt.Errorf("rotate invite code: reset counter: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("rotate invite code: commit: %w", err)
	}
	return code, nil
}

// Used returns how many successful registrations the current invite code has
// consumed. A missing or malformed counter is reported as 0.
func (r *InviteRepo) Used() (int, error) {
	var value string
	err := r.db.QueryRow(
		`SELECT value FROM settings WHERE key = ?`,
		inviteUsedKey,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get invite used: %w", err)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, nil // tolerate a malformed counter as zero
	}
	return n, nil
}

// Acquire records one use of the invite code, enforcing limit, and reports
// whether a slot was granted. When limit <= 0 the code is unlimited: the counter
// still advances (so admins can see total sign-ups) but a slot is always
// granted. Otherwise a slot is granted only while used < limit, via a single
// conditional UPDATE so concurrent registrations can never exceed the cap.
func (r *InviteRepo) Acquire(limit int) (bool, error) {
	// Ensure the counter row exists so the UPDATE below can match it.
	if _, err := r.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?, '0') ON CONFLICT(key) DO NOTHING`,
		inviteUsedKey,
	); err != nil {
		return false, fmt.Errorf("acquire invite: init counter: %w", err)
	}

	if limit <= 0 {
		if _, err := r.db.Exec(
			`UPDATE settings SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT) WHERE key = ?`,
			inviteUsedKey,
		); err != nil {
			return false, fmt.Errorf("acquire invite: bump counter: %w", err)
		}
		return true, nil
	}

	res, err := r.db.Exec(
		`UPDATE settings SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT)
		 WHERE key = ? AND CAST(value AS INTEGER) < ?`,
		inviteUsedKey, limit,
	)
	if err != nil {
		return false, fmt.Errorf("acquire invite: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("acquire invite: rows affected: %w", err)
	}
	return n == 1, nil
}

// Release returns a previously acquired slot. Registration acquires a slot
// before creating the user; if creation then fails, Release undoes the bump so a
// failed attempt does not permanently consume a slot. It floors at zero.
func (r *InviteRepo) Release() error {
	if _, err := r.db.Exec(
		`UPDATE settings SET value = CAST(CAST(value AS INTEGER) - 1 AS TEXT)
		 WHERE key = ? AND CAST(value AS INTEGER) > 0`,
		inviteUsedKey,
	); err != nil {
		return fmt.Errorf("release invite: %w", err)
	}
	return nil
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
