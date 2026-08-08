// Package book provides persistence, business logic, and HTTP handlers for
// comic books. Every book-specific query is scoped to the owning user so that
// one user can never read or mutate another user's books.
package book

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ErrNotFound is returned when a book does not exist, or exists but belongs to a
// different user. The two cases are deliberately indistinguishable so callers
// cannot probe for the existence of another user's books.
var ErrNotFound = errors.New("not found")

// defaultStyle is applied when a book is created without an explicit style.
const defaultStyle = "ghibli"

// bookColumns is the ordered column list used by every SELECT so the scan order
// stays in sync with scanBook.
const bookColumns = "id, user_id, title, cover_url, style, summary, is_public, created_at, updated_at"

// Repo persists and retrieves books backed by the books table. All methods that
// address a specific book require the owning user's ID and filter on it, which
// is the enforcement point for per-user data isolation.
type Repo struct {
	db *sql.DB
}

// NewRepo returns a Repo backed by the given database handle.
func NewRepo(d *sql.DB) *Repo {
	return &Repo{db: d}
}

// Create inserts a new book owned by userID. An empty style falls back to
// defaultStyle. The persisted book (including generated id and timestamps) is
// returned by reading the row back so callers receive DB-generated values.
func (r *Repo) Create(userID int64, title, style, summary string) (models.Book, error) {
	if style == "" {
		style = defaultStyle
	}

	res, err := r.db.Exec(
		`INSERT INTO books (user_id, title, style, summary) VALUES (?, ?, ?, ?)`,
		userID, title, style, summary,
	)
	if err != nil {
		return models.Book{}, fmt.Errorf("create book for user %d: %w", userID, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.Book{}, fmt.Errorf("create book for user %d: last insert id: %w", userID, err)
	}

	return r.Get(userID, id)
}

// ListByUser returns all books owned by userID, most recently created first.
// It never returns another user's books.
func (r *Repo) ListByUser(userID int64) ([]models.Book, error) {
	rows, err := r.db.Query(
		`SELECT `+bookColumns+` FROM books WHERE user_id = ? ORDER BY id DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list books for user %d: %w", userID, err)
	}
	defer rows.Close()

	books := make([]models.Book, 0)
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, fmt.Errorf("list books for user %d: %w", userID, err)
		}
		books = append(books, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list books for user %d: %w", userID, err)
	}
	return books, nil
}

// Get returns the book with bookID owned by userID. If the book does not exist
// or belongs to another user, it returns ErrNotFound.
func (r *Repo) Get(userID, bookID int64) (models.Book, error) {
	row := r.db.QueryRow(
		`SELECT `+bookColumns+` FROM books WHERE id = ? AND user_id = ?`,
		bookID, userID,
	)

	b, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Book{}, ErrNotFound
	}
	if err != nil {
		return models.Book{}, fmt.Errorf("get book %d for user %d: %w", bookID, userID, err)
	}
	return b, nil
}

// Update changes the title, style, and summary of the book owned by userID and
// refreshes updated_at. An empty style falls back to defaultStyle. It returns
// ErrNotFound if the book does not exist or belongs to another user.
func (r *Repo) Update(userID, bookID int64, title, style, summary string) (models.Book, error) {
	if style == "" {
		style = defaultStyle
	}

	res, err := r.db.Exec(
		`UPDATE books
		   SET title = ?, style = ?, summary = ?, updated_at = datetime('now')
		 WHERE id = ? AND user_id = ?`,
		title, style, summary, bookID, userID,
	)
	if err != nil {
		return models.Book{}, fmt.Errorf("update book %d for user %d: %w", bookID, userID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return models.Book{}, fmt.Errorf("update book %d for user %d: rows affected: %w", bookID, userID, err)
	}
	if affected == 0 {
		return models.Book{}, ErrNotFound
	}

	return r.Get(userID, bookID)
}

// SetCover sets the cover_url of the book owned by userID and refreshes
// updated_at. It returns ErrNotFound if the book does not exist or belongs to
// another user. The refreshed book row is returned.
func (r *Repo) SetCover(userID, bookID int64, coverURL string) (models.Book, error) {
	res, err := r.db.Exec(
		`UPDATE books
		   SET cover_url = ?, updated_at = datetime('now')
		 WHERE id = ? AND user_id = ?`,
		coverURL, bookID, userID,
	)
	if err != nil {
		return models.Book{}, fmt.Errorf("set cover for book %d for user %d: %w", bookID, userID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return models.Book{}, fmt.Errorf("set cover for book %d for user %d: rows affected: %w", bookID, userID, err)
	}
	if affected == 0 {
		return models.Book{}, ErrNotFound
	}

	return r.Get(userID, bookID)
}

// Delete removes the book owned by userID. It returns ErrNotFound if the book
// does not exist or belongs to another user.
func (r *Repo) Delete(userID, bookID int64) error {
	res, err := r.db.Exec(
		`DELETE FROM books WHERE id = ? AND user_id = ?`,
		bookID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete book %d for user %d: %w", bookID, userID, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete book %d for user %d: rows affected: %w", bookID, userID, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, letting scanBook serve
// single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

// scanBook reads one book row in bookColumns order.
func scanBook(s scanner) (models.Book, error) {
	var b models.Book
	if err := s.Scan(
		&b.ID, &b.UserID, &b.Title, &b.CoverURL, &b.Style,
		&b.Summary, &b.IsPublic, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return models.Book{}, err
	}
	return b, nil
}
