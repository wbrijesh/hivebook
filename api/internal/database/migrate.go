package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxdriver "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// newMigrator builds a golang-migrate instance over the embedded migrations,
// reusing an existing *sql.DB (opened with the pgx stdlib driver) so migrations
// run on the same connection as the app. Caller closes it with m.Close() when
// driving up/down directly (tests); the app path uses migrate() below.
func newMigrator(db *sql.DB) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("migration source: %w", err)
	}
	drv, err := pgxdriver.WithInstance(db, &pgxdriver.Config{})
	if err != nil {
		return nil, fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", drv)
	if err != nil {
		return nil, fmt.Errorf("migrator: %w", err)
	}
	return m, nil
}

// migrate applies all up migrations on startup. No-op when already current
// (golang-migrate, embedded — ADR-0017). The source/driver instances created
// here are tied to s.db, so we do not close the *sql.DB out from under the app.
func (s *service) migrate() error {
	m, err := newMigrator(s.db)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
