package db

import (
	"database/sql"
	"fmt"
	"strings"
)

// schemaStatements holds one CREATE TABLE statement per entry. The modernc
// SQLite driver executes a single statement per Exec, so migrations are applied
// one statement at a time. All statements use IF NOT EXISTS and are idempotent.
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  nickname TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
)`,
	`CREATE TABLE IF NOT EXISTS books (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  cover_url TEXT NOT NULL DEFAULT '',
  style TEXT NOT NULL DEFAULT 'ghibli',
  summary TEXT NOT NULL DEFAULT '',
  is_public INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)`,
	`CREATE TABLE IF NOT EXISTS characters (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  type TEXT NOT NULL DEFAULT 'character',
  name TEXT NOT NULL DEFAULT '',
  gender TEXT NOT NULL DEFAULT '',
  age TEXT NOT NULL DEFAULT '',
  personality TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
)`,
	`CREATE TABLE IF NOT EXISTS scenes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
)`,
	`CREATE TABLE IF NOT EXISTS chapters (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  book_id INTEGER NOT NULL,
  "order" INTEGER NOT NULL DEFAULT 0,
  title TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'draft',
  summary TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (book_id) REFERENCES books(id) ON DELETE CASCADE
)`,
	`CREATE TABLE IF NOT EXISTS panels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  chapter_id INTEGER NOT NULL,
  "order" INTEGER NOT NULL DEFAULT 0,
  caption TEXT NOT NULL DEFAULT '',
  character_ids TEXT NOT NULL DEFAULT '[]',
  scene_id INTEGER NOT NULL DEFAULT 0,
  image_prompt TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  location TEXT NOT NULL DEFAULT '',
  event TEXT NOT NULL DEFAULT '',
  char_expressions TEXT NOT NULL DEFAULT '{}',
  FOREIGN KEY (chapter_id) REFERENCES chapters(id) ON DELETE CASCADE
)`,
	`CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)`,
}

// alterStatements holds idempotent ADD COLUMN migrations applied AFTER the
// CREATE TABLE statements so pre-existing databases (created before a column was
// introduced) gain the new columns. Re-running an ADD COLUMN for an existing
// column raises a "duplicate column name" error, which Migrate tolerates.
var alterStatements = []string{
	`ALTER TABLE panels ADD COLUMN location TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE panels ADD COLUMN event TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE panels ADD COLUMN char_expressions TEXT NOT NULL DEFAULT '{}'`,
	`ALTER TABLE chapters ADD COLUMN summary TEXT NOT NULL DEFAULT ''`,
}

// Migrate creates all application tables if they do not already exist, then
// applies idempotent ADD COLUMN migrations for columns added to existing tables.
// It is safe to call repeatedly.
func Migrate(d *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := d.Exec(stmt); err != nil {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	for _, stmt := range alterStatements {
		if _, err := d.Exec(stmt); err != nil && !isDuplicateColumn(err) {
			return fmt.Errorf("exec migration: %w", err)
		}
	}
	return nil
}

// isDuplicateColumn reports whether err is SQLite's "duplicate column name"
// error, raised when an ADD COLUMN targets a column that already exists. This is
// the expected outcome for a fresh DB (where the column was created inline) and
// for a repeat migration, so it must not fail the run.
func isDuplicateColumn(err error) bool {
	return strings.Contains(err.Error(), "duplicate column name")
}
