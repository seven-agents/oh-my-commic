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

// Create inserts a new user with the given nickname and pre-computed password
// hash, returning the persisted user (including its generated ID and
// created_at timestamp). If the nickname already exists, the UNIQUE constraint
// on users.nickname causes an error to be returned.
func (r *UserRepo) Create(nickname, passwordHash string) (models.User, error) {
	res, err := r.db.Exec(
		`INSERT INTO users (nickname, password_hash) VALUES (?, ?)`,
		nickname, passwordHash,
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
		`SELECT id, nickname, password_hash, created_at FROM users WHERE nickname = ?`,
		nickname,
	)
	return scanUser(row, fmt.Sprintf("by nickname %q", nickname))
}

// byID returns the user with the given id.
func (r *UserRepo) byID(id int64) (models.User, error) {
	row := r.db.QueryRow(
		`SELECT id, nickname, password_hash, created_at FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row, fmt.Sprintf("by id %d", id))
}

// scanUser reads a single user row. ctx describes the lookup for error context.
func scanUser(row *sql.Row, ctx string) (models.User, error) {
	var u models.User
	if err := row.Scan(&u.ID, &u.Nickname, &u.PasswordHash, &u.CreatedAt); err != nil {
		return models.User{}, fmt.Errorf("fetch user %s: %w", ctx, err)
	}
	return u, nil
}
