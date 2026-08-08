// Package auth provides user persistence and (in later tasks) credential
// verification for the oh-my-commic backend.
package auth

import (
	"database/sql"
	"fmt"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// UserRepo persists and retrieves user accounts backed by the users table.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo returns a UserRepo backed by the given database handle.
func NewUserRepo(d *sql.DB) *UserRepo {
	return &UserRepo{db: d}
}

// Create inserts a new user with the given nickname, pre-computed password
// hash, and starting credit balance, returning the persisted user (including
// its generated ID and created_at timestamp). If the nickname already exists,
// the UNIQUE constraint on users.nickname causes an error to be returned.
func (r *UserRepo) Create(nickname, passwordHash string, credits int) (models.User, error) {
	res, err := r.db.Exec(
		`INSERT INTO users (nickname, password_hash, credits) VALUES (?, ?, ?)`,
		nickname, passwordHash, credits,
	)
	if err != nil {
		return models.User{}, fmt.Errorf("create user %q: %w", nickname, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.User{}, fmt.Errorf("create user %q: last insert id: %w", nickname, err)
	}

	// Read the row back so callers receive the DB-generated created_at value
	// rather than reconstructing it in Go.
	return r.byID(id)
}

// ByNickname returns the user with the given nickname. If no such user exists,
// a non-nil error is returned.
func (r *UserRepo) ByNickname(nickname string) (models.User, error) {
	row := r.db.QueryRow(
		`SELECT id, nickname, password_hash, created_at, credits FROM users WHERE nickname = ?`,
		nickname,
	)
	return scanUser(row, fmt.Sprintf("by nickname %q", nickname))
}

// ByID returns the user with the given id. It is the exported lookup used to
// resolve the currently authenticated user (e.g. GET /api/me).
func (r *UserRepo) ByID(id int64) (models.User, error) {
	return r.byID(id)
}

// byID returns the user with the given id.
func (r *UserRepo) byID(id int64) (models.User, error) {
	row := r.db.QueryRow(
		`SELECT id, nickname, password_hash, created_at, credits FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row, fmt.Sprintf("by id %d", id))
}

// Spend atomically deducts cost credits from the user's balance, but only if
// the balance is at least cost. It returns ok=true when exactly one row was
// updated (the charge succeeded) and ok=false when the balance was
// insufficient (no row updated). The guard and the deduction happen in a single
// UPDATE so concurrent spends can never overdraw the balance.
func (r *UserRepo) Spend(userID int64, cost int) (bool, error) {
	res, err := r.db.Exec(
		`UPDATE users SET credits = credits - ? WHERE id = ? AND credits >= ?`,
		cost, userID, cost,
	)
	if err != nil {
		return false, fmt.Errorf("spend %d credits for user %d: %w", cost, userID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("spend %d credits for user %d: rows affected: %w", cost, userID, err)
	}
	return n == 1, nil
}

// Refund returns amount credits to the user's balance. It is used to undo a
// prior Spend when the paid-for image generation ultimately fails.
func (r *UserRepo) Refund(userID int64, amount int) error {
	if _, err := r.db.Exec(
		`UPDATE users SET credits = credits + ? WHERE id = ?`,
		amount, userID,
	); err != nil {
		return fmt.Errorf("refund %d credits to user %d: %w", amount, userID, err)
	}
	return nil
}

// Credits returns the user's current credit balance.
func (r *UserRepo) Credits(userID int64) (int, error) {
	u, err := r.byID(userID)
	if err != nil {
		return 0, err
	}
	return u.Credits, nil
}

// scanUser reads a single user row. ctx describes the lookup for error context.
func scanUser(row *sql.Row, ctx string) (models.User, error) {
	var u models.User
	if err := row.Scan(&u.ID, &u.Nickname, &u.PasswordHash, &u.CreatedAt, &u.Credits); err != nil {
		return models.User{}, fmt.Errorf("fetch user %s: %w", ctx, err)
	}
	return u, nil
}
