// Package db opens the SQLite database connection and runs schema migrations.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// Open opens (or creates) the SQLite database at path, enables foreign key
// enforcement, and applies all schema migrations. The returned *sql.DB is
// ready for use; the caller is responsible for closing it.
func Open(path string) (*sql.DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}

	if _, err := d.Exec("PRAGMA foreign_keys=ON"); err != nil {
		d.Close()
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}

	if err := Migrate(d); err != nil {
		d.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}
