// Package chapter provides persistence and business logic for the chapters of a
// comic book. A chapter carries an ordering position within its book and a
// status that advances through a fixed state machine
// (draft → storyboarding → rendering → done). Every service operation is gated
// by book ownership so one user can never read or mutate another user's
// chapters.
package chapter

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ErrNotFound is returned when a chapter does not exist, or when the caller is
// not allowed to access it. The two cases are deliberately indistinguishable so
// callers cannot probe for the existence of another user's chapters.
var ErrNotFound = errors.New("not found")

// ErrInvalidStatus is returned when a status transition is not permitted by the
// chapter state machine (see allowedTransitions in service.go).
var ErrInvalidStatus = errors.New("invalid status transition")

// statusDraft is the initial status assigned to every newly created chapter.
const statusDraft = "draft"

// chapterColumns is the ordered column list used by every chapter SELECT so the
// scan order stays in sync with scanChapter. "order" is a SQL reserved word and
// is always quoted.
const chapterColumns = `id, book_id, "order", title, status, summary, is_cover, conversation, panel_count, created_at`

// coverTitle is the fixed title assigned to a book's cover chapter.
const coverTitle = "封面"

// Repo performs pure data operations on the chapters table. It is keyed by
// book_id / id and does NOT enforce ownership; that is the Service's
// responsibility.
type Repo struct {
	db *sql.DB
}

// NewRepo returns a Repo backed by the given database handle.
func NewRepo(d *sql.DB) *Repo {
	return &Repo{db: d}
}

// Create inserts a chapter under bookID with status "draft". Its order is the
// current maximum order within the book plus one, so the first chapter of a book
// gets order 1 and subsequent chapters increment consecutively. The persisted
// row (including generated id and timestamp) is read back and returned.
func (r *Repo) Create(bookID int64, title string) (models.Chapter, error) {
	// COALESCE(MAX("order"), 0) + 1 yields 1 for an empty book and max+1
	// otherwise, computed atomically within the INSERT.
	res, err := r.db.Exec(
		`INSERT INTO chapters (book_id, "order", title, status, is_cover)
		 VALUES (?, (SELECT COALESCE(MAX("order"), 0) + 1 FROM chapters WHERE book_id = ?), ?, ?, 0)`,
		bookID, bookID, title, statusDraft,
	)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("create chapter for book %d: %w", bookID, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.Chapter{}, fmt.Errorf("create chapter for book %d: last insert id: %w", bookID, err)
	}
	return r.Get(id)
}

// FindCover returns the book's cover chapter (is_cover=1), or ErrNotFound if the
// book has none. A book has at most one cover chapter; LIMIT 1 guards against any
// stray duplicate.
func (r *Repo) FindCover(bookID int64) (models.Chapter, error) {
	row := r.db.QueryRow(
		`SELECT `+chapterColumns+` FROM chapters WHERE book_id = ? AND is_cover = 1 LIMIT 1`,
		bookID,
	)
	c, err := scanChapter(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Chapter{}, ErrNotFound
	}
	if err != nil {
		return models.Chapter{}, fmt.Errorf("find cover for book %d: %w", bookID, err)
	}
	return c, nil
}

// CreateCover inserts the book's special cover chapter: order 0, is_cover=1,
// title "封面", status "draft". The persisted row is read back and returned.
// Callers must ensure the book does not already have a cover chapter (see
// Service.EnsureCover); this method does not itself enforce single-cover.
func (r *Repo) CreateCover(bookID int64) (models.Chapter, error) {
	res, err := r.db.Exec(
		`INSERT INTO chapters (book_id, "order", title, status, is_cover)
		 VALUES (?, 0, ?, ?, 1)`,
		bookID, coverTitle, statusDraft,
	)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("create cover chapter for book %d: %w", bookID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Chapter{}, fmt.Errorf("create cover chapter for book %d: last insert id: %w", bookID, err)
	}
	return r.Get(id)
}

// ListByBook returns all chapters under bookID ordered by their order position.
func (r *Repo) ListByBook(bookID int64) ([]models.Chapter, error) {
	rows, err := r.db.Query(
		`SELECT `+chapterColumns+` FROM chapters WHERE book_id = ? ORDER BY "order"`,
		bookID,
	)
	if err != nil {
		return nil, fmt.Errorf("list chapters for book %d: %w", bookID, err)
	}
	defer rows.Close()

	chapters := make([]models.Chapter, 0)
	for rows.Next() {
		c, err := scanChapter(rows)
		if err != nil {
			return nil, fmt.Errorf("list chapters for book %d: %w", bookID, err)
		}
		chapters = append(chapters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list chapters for book %d: %w", bookID, err)
	}
	return chapters, nil
}

// Get returns the chapter with id, or ErrNotFound. It is used for the ownership
// re-check on per-chapter operations.
func (r *Repo) Get(id int64) (models.Chapter, error) {
	row := r.db.QueryRow(
		`SELECT `+chapterColumns+` FROM chapters WHERE id = ?`,
		id,
	)
	c, err := scanChapter(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Chapter{}, ErrNotFound
	}
	if err != nil {
		return models.Chapter{}, fmt.Errorf("get chapter %d: %w", id, err)
	}
	return c, nil
}

// SetStatus overwrites the status of the chapter with id and returns the
// refreshed row. It returns ErrNotFound if no such chapter exists. Transition
// validation is the Service's responsibility.
func (r *Repo) SetStatus(id int64, status string) (models.Chapter, error) {
	res, err := r.db.Exec(
		`UPDATE chapters SET status = ? WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set status for chapter %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set status for chapter %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return models.Chapter{}, ErrNotFound
	}
	return r.Get(id)
}

// SetSummary overwrites the summary of the chapter with id and returns the
// refreshed row. It returns ErrNotFound if no such chapter exists. Ownership is
// the Service's responsibility.
func (r *Repo) SetSummary(id int64, summary string) (models.Chapter, error) {
	res, err := r.db.Exec(
		`UPDATE chapters SET summary = ? WHERE id = ?`,
		summary, id,
	)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set summary for chapter %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set summary for chapter %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return models.Chapter{}, ErrNotFound
	}
	return r.Get(id)
}

// SetConversation overwrites the persisted storyboard-chat conversation and the
// target panel_count of the chapter with id, returning the refreshed row. conv
// is serialized to a JSON array TEXT. It returns ErrNotFound if no such chapter
// exists. Ownership is the Service's responsibility.
func (r *Repo) SetConversation(id int64, conv []models.ConversationMsg, panelCount int) (models.Chapter, error) {
	convJSON, err := marshalConversation(conv)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set conversation for chapter %d: %w", id, err)
	}
	res, err := r.db.Exec(
		`UPDATE chapters SET conversation = ?, panel_count = ? WHERE id = ?`,
		convJSON, panelCount, id,
	)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set conversation for chapter %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return models.Chapter{}, fmt.Errorf("set conversation for chapter %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return models.Chapter{}, ErrNotFound
	}
	return r.Get(id)
}

// Delete removes the chapter with id. It returns ErrNotFound if no such chapter
// exists. Deleting a chapter cascades to its panels via the
// FOREIGN KEY (chapter_id) ... ON DELETE CASCADE constraint on the panels table.
// Ownership is the Service's responsibility.
func (r *Repo) Delete(id int64) error {
	res, err := r.db.Exec(
		`DELETE FROM chapters WHERE id = ?`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete chapter %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete chapter %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, letting scanChapter serve
// single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

// scanChapter reads one chapter row in chapterColumns order, deserializing the
// conversation JSON column into a []models.ConversationMsg.
func scanChapter(s scanner) (models.Chapter, error) {
	var (
		c    models.Chapter
		conv string
	)
	if err := s.Scan(
		&c.ID, &c.BookID, &c.Order, &c.Title, &c.Status, &c.Summary, &c.IsCover, &conv, &c.PanelCount, &c.CreatedAt,
	); err != nil {
		return models.Chapter{}, err
	}
	parsed, err := unmarshalConversation(conv)
	if err != nil {
		return models.Chapter{}, fmt.Errorf("decode conversation for chapter %d: %w", c.ID, err)
	}
	c.Conversation = parsed
	return c, nil
}

// marshalConversation serializes a conversation history into a JSON array TEXT.
// A nil or empty slice serializes to "[]" so the stored column is always valid
// JSON.
func marshalConversation(conv []models.ConversationMsg) (string, error) {
	if conv == nil {
		conv = []models.ConversationMsg{}
	}
	b, err := json.Marshal(conv)
	if err != nil {
		return "", fmt.Errorf("marshal conversation: %w", err)
	}
	return string(b), nil
}

// unmarshalConversation deserializes a JSON array TEXT back into a conversation
// history. An empty or whitespace-only string yields an empty slice rather than
// an error.
func unmarshalConversation(s string) ([]models.ConversationMsg, error) {
	out := []models.ConversationMsg{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("unmarshal conversation: %w", err)
	}
	return out, nil
}
