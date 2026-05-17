package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migsqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/unkabas/dbil/internal/store/migrations"
)

// Apply runs every pending migration in the embedded migrations FS against db.
// Idempotent: applying twice on the same DB is a no-op.
func Apply(db *sql.DB) error {
	src, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("store: migration source: %w", err)
	}
	drv, err := migsqlite.WithInstance(db, &migsqlite.Config{})
	if err != nil {
		return fmt.Errorf("store: migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", drv)
	if err != nil {
		return fmt.Errorf("store: migrate new: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("store: migrate up: %w", err)
	}
	return nil
}
