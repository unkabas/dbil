// Package store wraps DBil's SQLite state: connection pool, migrations,
// and typed repositories for users and audit log.
//
// The state file itself is not file-level encrypted (no SQLCipher / CGO).
// All sensitive columns (credentials, audit details) are encrypted at the
// application layer via the MK→DEK envelope in internal/crypto, so a leaked
// DB file without the master key yields ciphertext for every sensitive value.
package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// driverName is the sql driver registered by modernc.org/sqlite.
const driverName = "sqlite"

// Open returns a connection pool for the SQLite file at path. The DSN sets
// our standard PRAGMAs: busy_timeout=5s, WAL journal mode, foreign keys on.
func Open(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("store: abs path: %w", err)
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(on)")
	dsn := "file:" + abs + "?" + q.Encode()

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping %s: %w", path, err)
	}
	return db, nil
}

// Close closes the connection pool. A nil receiver is a safe no-op.
func Close(db *sql.DB) error {
	if db == nil {
		return nil
	}
	return db.Close()
}
