// Package auth provides user persistence and (in later tasks) credential
// verification for the oh-my-commic backend.
package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// Sentinel errors returned by Create when a unique constraint is violated. They
// let callers map the failure to a field-specific message without inspecting
// the driver error.
var (
	// ErrUsernameTaken is returned by Create when the username already exists.
	ErrUsernameTaken = errors.New("用户名已被占用")
	// ErrEmailTaken is returned by Create when the email is already registered.
	ErrEmailTaken = errors.New("邮箱已注册")
)

// userColumns is the canonical, ordered column list for every user SELECT. It
// must stay in lock-step with scanUser's Scan destinations.
const userColumns = `id, username, email, password_hash, role, nickname, age, gender, avatar_url, credits, created_at`

// UserRepo persists and retrieves user accounts backed by the users table.
type UserRepo struct {
	db *sql.DB
}

// NewUserRepo returns a UserRepo backed by the given database handle.
func NewUserRepo(d *sql.DB) *UserRepo {
	return &UserRepo{db: d}
}

// NewUser carries the fields needed to insert a new account. It is shared by
// registration and seeding. Username/Email participate in partial unique
// indexes; an empty value is permitted (legacy accounts) and never collides.
type NewUser struct {
	Username     string
	Email        string
	PasswordHash string
	Nickname     string
	Role         string
	Credits      int
}

// Create inserts a new user and returns the persisted row (including its
// generated ID and created_at timestamp). A duplicate non-empty username or
// email violates the partial unique index idx_users_username / idx_users_email;
// Create maps those to ErrUsernameTaken / ErrEmailTaken respectively.
func (r *UserRepo) Create(u NewUser) (models.User, error) {
	res, err := r.db.Exec(
		`INSERT INTO users (username, email, password_hash, role, nickname, credits)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.PasswordHash, u.Role, u.Nickname, u.Credits,
	)
	if err != nil {
		if mapped := mapUniqueErr(err); mapped != nil {
			return models.User{}, mapped
		}
		return models.User{}, fmt.Errorf("create user %q: %w", u.Username, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.User{}, fmt.Errorf("create user %q: last insert id: %w", u.Username, err)
	}

	// Read the row back so callers receive the DB-generated created_at value
	// rather than reconstructing it in Go.
	return r.byID(id)
}

// mapUniqueErr inspects a driver error and returns the matching sentinel when it
// is a UNIQUE constraint violation on username or email, else nil. It matches on
// the constraint text ("...username" / "...email") so it works for both the
// index name (idx_users_username) and the column name.
func mapUniqueErr(err error) error {
	msg := err.Error()
	if !strings.Contains(msg, "UNIQUE constraint failed") {
		return nil
	}
	switch {
	case strings.Contains(msg, "username"):
		return ErrUsernameTaken
	case strings.Contains(msg, "email"):
		return ErrEmailTaken
	default:
		return nil
	}
}

// ByUsername returns the user with the given username. If no such user exists,
// a non-nil error is returned. It is the lookup used to resolve a login.
func (r *UserRepo) ByUsername(username string) (models.User, error) {
	row := r.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE username = ?`,
		username,
	)
	return scanUser(row, fmt.Sprintf("by username %q", username))
}

// ByID returns the user with the given id. It is the exported lookup used to
// resolve the currently authenticated user (e.g. GET /api/me).
func (r *UserRepo) ByID(id int64) (models.User, error) {
	return r.byID(id)
}

// byID returns the user with the given id.
func (r *UserRepo) byID(id int64) (models.User, error) {
	row := r.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE id = ?`,
		id,
	)
	return scanUser(row, fmt.Sprintf("by id %d", id))
}

// UpdateProfile updates the editable profile fields (display nickname, age,
// gender) for the given user and returns the refreshed row. Username, email and
// role are immutable here and left untouched.
func (r *UserRepo) UpdateProfile(userID int64, nickname string, age int, gender string) (models.User, error) {
	if _, err := r.db.Exec(
		`UPDATE users SET nickname = ?, age = ?, gender = ? WHERE id = ?`,
		nickname, age, gender, userID,
	); err != nil {
		return models.User{}, fmt.Errorf("update profile for user %d: %w", userID, err)
	}
	return r.byID(userID)
}

// SetAvatar updates the avatar URL for the given user and returns the refreshed
// row.
func (r *UserRepo) SetAvatar(userID int64, avatarURL string) (models.User, error) {
	if _, err := r.db.Exec(
		`UPDATE users SET avatar_url = ? WHERE id = ?`,
		avatarURL, userID,
	); err != nil {
		return models.User{}, fmt.Errorf("set avatar for user %d: %w", userID, err)
	}
	return r.byID(userID)
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

// Count returns the total number of registered users. It backs the
// omc_registered_users Prometheus gauge, so it is a single cheap aggregate.
func (r *UserRepo) Count() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

// scanUser reads a single user row. ctx describes the lookup for error context.
// The Scan destinations must stay in lock-step with userColumns.
func scanUser(row *sql.Row, ctx string) (models.User, error) {
	var u models.User
	if err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role,
		&u.Nickname, &u.Age, &u.Gender, &u.AvatarURL, &u.Credits, &u.CreatedAt,
	); err != nil {
		return models.User{}, fmt.Errorf("fetch user %s: %w", ctx, err)
	}
	return u, nil
}
