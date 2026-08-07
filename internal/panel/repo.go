// Package panel provides persistence and business logic for the comic frames
// (panels) of a chapter. Panels carry an ordering position within their chapter
// and a rendering status. The bulk ReplaceForChapter operation atomically swaps
// the entire panel set of a chapter, reassigning 0-based order values so the
// stored ordering always matches the confirmed storyboard sequence. Every
// service operation is gated by chapter ownership so one user can never read or
// mutate another user's panels.
package panel

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ErrNotFound is returned when a panel does not exist, or when the caller is not
// allowed to access it. The two cases are deliberately indistinguishable so
// callers cannot probe for the existence of another user's panels or chapters.
var ErrNotFound = errors.New("not found")

// statusPending is the initial status assigned to every newly inserted panel.
const statusPending = "pending"

// statusDone marks a panel whose image has been generated.
const statusDone = "done"

// panelColumns is the ordered column list used by every panel SELECT so the scan
// order stays in sync with scanPanel. "order" is a SQL reserved word and is
// always quoted.
const panelColumns = `id, chapter_id, "order", caption, character_ids, scene_id, image_prompt, image_url, status`

// Repo performs pure data operations on the panels table. It is keyed by
// chapter_id / id and does NOT enforce ownership; that is the Service's
// responsibility.
type Repo struct {
	db *sql.DB
}

// NewRepo returns a Repo backed by the given database handle.
func NewRepo(d *sql.DB) *Repo {
	return &Repo{db: d}
}

// ReplaceForChapter atomically replaces every panel under chapterID with the
// given panels. Within a single transaction it deletes the chapter's existing
// panels then inserts the new ones in slice order, assigning "order" values
// 0, 1, 2, … (0-based). CharacterIDs is serialized to a JSON array TEXT (nil
// becomes "[]"). The freshly inserted panels — with generated ids and assigned
// orders — are returned. On any error the transaction is rolled back so the
// chapter's panels are left unchanged.
func (r *Repo) ReplaceForChapter(chapterID int64, panels []models.Panel) ([]models.Panel, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("replace panels for chapter %d: begin: %w", chapterID, err)
	}
	// Rollback is a no-op after a successful Commit, so this defer is safe.
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM panels WHERE chapter_id = ?`, chapterID); err != nil {
		return nil, fmt.Errorf("replace panels for chapter %d: delete: %w", chapterID, err)
	}

	out := make([]models.Panel, 0, len(panels))
	for i, p := range panels {
		ids, err := marshalIDs(p.CharacterIDs)
		if err != nil {
			return nil, fmt.Errorf("replace panels for chapter %d: marshal character ids: %w", chapterID, err)
		}
		order := int64(i)
		res, err := tx.Exec(
			`INSERT INTO panels (chapter_id, "order", caption, character_ids, scene_id, image_prompt, image_url, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			chapterID, order, p.Caption, ids, p.SceneID, p.ImagePrompt, p.ImageURL, statusPending,
		)
		if err != nil {
			return nil, fmt.Errorf("replace panels for chapter %d: insert: %w", chapterID, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("replace panels for chapter %d: last insert id: %w", chapterID, err)
		}

		inserted := p
		inserted.ID = id
		inserted.ChapterID = chapterID
		inserted.Order = int(order)
		inserted.CharacterIDs = normalizeIDs(p.CharacterIDs)
		inserted.Status = statusPending
		out = append(out, inserted)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("replace panels for chapter %d: commit: %w", chapterID, err)
	}
	return out, nil
}

// ListByChapter returns all panels under chapterID ordered by their 0-based
// order position. character_ids JSON is deserialized back into []int64.
func (r *Repo) ListByChapter(chapterID int64) ([]models.Panel, error) {
	rows, err := r.db.Query(
		`SELECT `+panelColumns+` FROM panels WHERE chapter_id = ? ORDER BY "order"`,
		chapterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list panels for chapter %d: %w", chapterID, err)
	}
	defer rows.Close()

	panels := make([]models.Panel, 0)
	for rows.Next() {
		p, err := scanPanel(rows)
		if err != nil {
			return nil, fmt.Errorf("list panels for chapter %d: %w", chapterID, err)
		}
		panels = append(panels, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list panels for chapter %d: %w", chapterID, err)
	}
	return panels, nil
}

// Get returns the panel with id, or ErrNotFound. It is used for the ownership
// re-check on per-panel operations.
func (r *Repo) Get(id int64) (models.Panel, error) {
	row := r.db.QueryRow(
		`SELECT `+panelColumns+` FROM panels WHERE id = ?`,
		id,
	)
	p, err := scanPanel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Panel{}, ErrNotFound
	}
	if err != nil {
		return models.Panel{}, fmt.Errorf("get panel %d: %w", id, err)
	}
	return p, nil
}

// Update overwrites the editable fields of the panel with id — caption,
// character_ids (JSON), scene_id and image_prompt — and returns the refreshed
// row. It returns ErrNotFound if no such panel exists.
func (r *Repo) Update(id int64, p models.Panel) (models.Panel, error) {
	ids, err := marshalIDs(p.CharacterIDs)
	if err != nil {
		return models.Panel{}, fmt.Errorf("update panel %d: marshal character ids: %w", id, err)
	}
	res, err := r.db.Exec(
		`UPDATE panels SET caption = ?, character_ids = ?, scene_id = ?, image_prompt = ? WHERE id = ?`,
		p.Caption, ids, p.SceneID, p.ImagePrompt, id,
	)
	if err != nil {
		return models.Panel{}, fmt.Errorf("update panel %d: %w", id, err)
	}
	if err := ensureAffected(res, id); err != nil {
		return models.Panel{}, err
	}
	return r.Get(id)
}

// SetImage stores url as the panel's image_url and advances its status to
// "done". It returns the refreshed row, or ErrNotFound if no such panel exists.
func (r *Repo) SetImage(id int64, url string) (models.Panel, error) {
	res, err := r.db.Exec(
		`UPDATE panels SET image_url = ?, status = ? WHERE id = ?`,
		url, statusDone, id,
	)
	if err != nil {
		return models.Panel{}, fmt.Errorf("set image for panel %d: %w", id, err)
	}
	if err := ensureAffected(res, id); err != nil {
		return models.Panel{}, err
	}
	return r.Get(id)
}

// SetStatus overwrites the status of the panel with id and returns the refreshed
// row. It returns ErrNotFound if no such panel exists.
func (r *Repo) SetStatus(id int64, status string) (models.Panel, error) {
	res, err := r.db.Exec(
		`UPDATE panels SET status = ? WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return models.Panel{}, fmt.Errorf("set status for panel %d: %w", id, err)
	}
	if err := ensureAffected(res, id); err != nil {
		return models.Panel{}, err
	}
	return r.Get(id)
}

// ensureAffected maps a zero-rows UPDATE to ErrNotFound.
func ensureAffected(res sql.Result, id int64) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("panel %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, letting scanPanel serve
// single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

// scanPanel reads one panel row in panelColumns order, deserializing the
// character_ids JSON column into a []int64.
func scanPanel(s scanner) (models.Panel, error) {
	var (
		p   models.Panel
		ids string
	)
	if err := s.Scan(
		&p.ID, &p.ChapterID, &p.Order, &p.Caption, &ids, &p.SceneID, &p.ImagePrompt, &p.ImageURL, &p.Status,
	); err != nil {
		return models.Panel{}, err
	}
	parsed, err := unmarshalIDs(ids)
	if err != nil {
		return models.Panel{}, fmt.Errorf("decode character ids for panel %d: %w", p.ID, err)
	}
	p.CharacterIDs = parsed
	return p, nil
}

// marshalIDs serializes a slice of character ids into a JSON array TEXT. A nil
// or empty slice serializes to "[]" so the stored column is always valid JSON.
func marshalIDs(ids []int64) (string, error) {
	b, err := json.Marshal(normalizeIDs(ids))
	if err != nil {
		return "", fmt.Errorf("marshal character ids: %w", err)
	}
	return string(b), nil
}

// unmarshalIDs deserializes a JSON array TEXT back into a []int64. An empty or
// whitespace-only string is treated as an empty slice rather than an error.
func unmarshalIDs(s string) ([]int64, error) {
	if strings.TrimSpace(s) == "" {
		return []int64{}, nil
	}
	ids := []int64{}
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return nil, fmt.Errorf("unmarshal character ids: %w", err)
	}
	return ids, nil
}

// normalizeIDs returns a non-nil slice, converting nil to an empty slice so JSON
// serialization yields "[]" and the model never carries a nil CharacterIDs.
func normalizeIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}
