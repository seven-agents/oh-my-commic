// Package asset provides persistence and business logic for the reusable assets
// of a comic book: characters (cast members) and scenes (settings). Every
// service operation is gated by book ownership so one user can never read or
// mutate assets belonging to another user's book.
package asset

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/seven-agents/oh-my-commic/internal/models"
)

// ErrNotFound is returned when a character or scene does not exist, or when the
// caller is not allowed to access it. The two cases are deliberately
// indistinguishable so callers cannot probe for the existence of another user's
// assets.
var ErrNotFound = errors.New("not found")

// defaultCharacterType is applied when a character is created without an
// explicit type.
const defaultCharacterType = "character"

// characterColumns is the ordered column list used by every character SELECT so
// the scan order stays in sync with scanCharacter.
const characterColumns = "id, book_id, type, name, gender, age, personality, description, image_url"

// sceneColumns is the ordered column list used by every scene SELECT so the
// scan order stays in sync with scanScene.
const sceneColumns = "id, book_id, name, description, image_url"

// Repo performs pure data operations on the characters and scenes tables. It is
// keyed by book_id / id and does NOT enforce ownership; that is the Service's
// responsibility.
type Repo struct {
	db *sql.DB
}

// NewRepo returns a Repo backed by the given database handle.
func NewRepo(d *sql.DB) *Repo {
	return &Repo{db: d}
}

// CreateCharacter inserts a character under bookID. An empty type falls back to
// defaultCharacterType. The persisted character (including generated id) is read
// back and returned.
func (r *Repo) CreateCharacter(bookID int64, c models.Character) (models.Character, error) {
	if c.Type == "" {
		c.Type = defaultCharacterType
	}

	res, err := r.db.Exec(
		`INSERT INTO characters (book_id, type, name, gender, age, personality, description, image_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		bookID, c.Type, c.Name, c.Gender, c.Age, c.Personality, c.Description, c.ImageURL,
	)
	if err != nil {
		return models.Character{}, fmt.Errorf("create character for book %d: %w", bookID, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return models.Character{}, fmt.Errorf("create character for book %d: last insert id: %w", bookID, err)
	}
	return r.GetCharacter(id)
}

// ListCharacters returns all characters under bookID ordered by id ascending.
func (r *Repo) ListCharacters(bookID int64) ([]models.Character, error) {
	rows, err := r.db.Query(
		`SELECT `+characterColumns+` FROM characters WHERE book_id = ? ORDER BY id`,
		bookID,
	)
	if err != nil {
		return nil, fmt.Errorf("list characters for book %d: %w", bookID, err)
	}
	defer rows.Close()

	characters := make([]models.Character, 0)
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, fmt.Errorf("list characters for book %d: %w", bookID, err)
		}
		characters = append(characters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list characters for book %d: %w", bookID, err)
	}
	return characters, nil
}

// GetCharacter returns the character with id, or ErrNotFound. It is used for the
// ownership re-check on update and delete.
func (r *Repo) GetCharacter(id int64) (models.Character, error) {
	row := r.db.QueryRow(
		`SELECT `+characterColumns+` FROM characters WHERE id = ?`,
		id,
	)
	c, err := scanCharacter(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Character{}, ErrNotFound
	}
	if err != nil {
		return models.Character{}, fmt.Errorf("get character %d: %w", id, err)
	}
	return c, nil
}

// UpdateCharacter overwrites the mutable fields of the character with id and
// returns the refreshed row. An empty type falls back to defaultCharacterType.
// It returns ErrNotFound if no such character exists.
func (r *Repo) UpdateCharacter(id int64, c models.Character) (models.Character, error) {
	if c.Type == "" {
		c.Type = defaultCharacterType
	}

	res, err := r.db.Exec(
		`UPDATE characters
		    SET type = ?, name = ?, gender = ?, age = ?, personality = ?, description = ?, image_url = ?
		  WHERE id = ?`,
		c.Type, c.Name, c.Gender, c.Age, c.Personality, c.Description, c.ImageURL, id,
	)
	if err != nil {
		return models.Character{}, fmt.Errorf("update character %d: %w", id, err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return models.Character{}, fmt.Errorf("update character %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return models.Character{}, ErrNotFound
	}
	return r.GetCharacter(id)
}

// DeleteCharacter removes the character with id, or returns ErrNotFound.
func (r *Repo) DeleteCharacter(id int64) error {
	res, err := r.db.Exec(`DELETE FROM characters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete character %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete character %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateScene inserts a scene under bookID and returns the persisted row.
func (r *Repo) CreateScene(bookID int64, s models.Scene) (models.Scene, error) {
	res, err := r.db.Exec(
		`INSERT INTO scenes (book_id, name, description, image_url) VALUES (?, ?, ?, ?)`,
		bookID, s.Name, s.Description, s.ImageURL,
	)
	if err != nil {
		return models.Scene{}, fmt.Errorf("create scene for book %d: %w", bookID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.Scene{}, fmt.Errorf("create scene for book %d: last insert id: %w", bookID, err)
	}
	return r.GetScene(id)
}

// ListScenes returns all scenes under bookID ordered by id ascending.
func (r *Repo) ListScenes(bookID int64) ([]models.Scene, error) {
	rows, err := r.db.Query(
		`SELECT `+sceneColumns+` FROM scenes WHERE book_id = ? ORDER BY id`,
		bookID,
	)
	if err != nil {
		return nil, fmt.Errorf("list scenes for book %d: %w", bookID, err)
	}
	defer rows.Close()

	scenes := make([]models.Scene, 0)
	for rows.Next() {
		s, err := scanScene(rows)
		if err != nil {
			return nil, fmt.Errorf("list scenes for book %d: %w", bookID, err)
		}
		scenes = append(scenes, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list scenes for book %d: %w", bookID, err)
	}
	return scenes, nil
}

// GetScene returns the scene with id, or ErrNotFound. It is used for the
// ownership re-check on update and delete.
func (r *Repo) GetScene(id int64) (models.Scene, error) {
	row := r.db.QueryRow(
		`SELECT `+sceneColumns+` FROM scenes WHERE id = ?`,
		id,
	)
	s, err := scanScene(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Scene{}, ErrNotFound
	}
	if err != nil {
		return models.Scene{}, fmt.Errorf("get scene %d: %w", id, err)
	}
	return s, nil
}

// UpdateScene overwrites the mutable fields of the scene with id and returns the
// refreshed row. It returns ErrNotFound if no such scene exists.
func (r *Repo) UpdateScene(id int64, s models.Scene) (models.Scene, error) {
	res, err := r.db.Exec(
		`UPDATE scenes SET name = ?, description = ?, image_url = ? WHERE id = ?`,
		s.Name, s.Description, s.ImageURL, id,
	)
	if err != nil {
		return models.Scene{}, fmt.Errorf("update scene %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return models.Scene{}, fmt.Errorf("update scene %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return models.Scene{}, ErrNotFound
	}
	return r.GetScene(id)
}

// DeleteScene removes the scene with id, or returns ErrNotFound.
func (r *Repo) DeleteScene(id int64) error {
	res, err := r.db.Exec(`DELETE FROM scenes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete scene %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete scene %d: rows affected: %w", id, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, letting the scan helpers
// serve single-row and multi-row queries.
type scanner interface {
	Scan(dest ...any) error
}

// scanCharacter reads one character row in characterColumns order.
func scanCharacter(s scanner) (models.Character, error) {
	var c models.Character
	if err := s.Scan(
		&c.ID, &c.BookID, &c.Type, &c.Name, &c.Gender,
		&c.Age, &c.Personality, &c.Description, &c.ImageURL,
	); err != nil {
		return models.Character{}, err
	}
	return c, nil
}

// scanScene reads one scene row in sceneColumns order.
func scanScene(s scanner) (models.Scene, error) {
	var sc models.Scene
	if err := s.Scan(
		&sc.ID, &sc.BookID, &sc.Name, &sc.Description, &sc.ImageURL,
	); err != nil {
		return models.Scene{}, err
	}
	return sc, nil
}
