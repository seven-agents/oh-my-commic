// Package db opens the SQLite database connection and runs schema migrations.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// Open opens (or creates) the SQLite database at path, enables foreign key
// enforcement on every pooled connection, and applies all schema migrations.
// The returned *sql.DB is ready for use; the caller is responsible for closing
// it.
//
// foreign_keys is a per-connection pragma, so it is set through the DSN
// (?_pragma=foreign_keys(1)) rather than a single d.Exec — the modernc driver
// applies DSN pragmas to each physical connection the pool opens, ensuring
// ON DELETE CASCADE fires under concurrent handlers.
func Open(path string) (*sql.DB, error) {
	dsn, inMemory := buildDSN(path)

	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}

	// A bare ":memory:" gives each pooled connection its own empty database,
	// which would hide migrations from later connections. Pin the pool to a
	// single connection for in-memory databases so all queries share one DB.
	if inMemory {
		d.SetMaxOpenConns(1)
	}

	if err := Migrate(d); err != nil {
		d.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

// buildDSN appends per-connection pragmas to the given path and reports whether
// the target is an in-memory database. It handles plain file paths, ":memory:",
// and paths that already carry query parameters.
//
// Pragmas applied to every connection:
//   - foreign_keys(1): enforce FK constraints / ON DELETE CASCADE.
//   - busy_timeout(5000): wait up to 5s instead of failing with SQLITE_BUSY
//     when another pooled connection holds the write lock.
func buildDSN(path string) (dsn string, inMemory bool) {
	const pragmas = "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

	inMemory = strings.Contains(path, ":memory:") ||
		strings.Contains(path, "mode=memory")

	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + pragmas, inMemory
}
