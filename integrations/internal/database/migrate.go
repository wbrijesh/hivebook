package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxdriver "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// newMigrator builds a golang-migrate instance over the embedded migrations,
// bound to the given *sql.DB. Callers own the *sql.DB and close the migrator with
// m.Close() when done — which also closes that connection, so never pass a shared
// pool here.
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

// applyMigrations opens a short-lived connection of its own, applies all up
// migrations, and closes everything when done — never touching the app pool, so
// closing leaks nothing across retries. No-op when already current (ADR-0017).
func applyMigrations(cfg Config) error {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer db.Close()

	m, err := newMigrator(db)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// migrateWithRetry runs applyMigrations until it succeeds or ctx expires —
// Postgres may not be up yet on first boot. The wait is context-aware.
func migrateWithRetry(ctx context.Context, cfg Config) error {
	const retryInterval = 2 * time.Second
	for {
		err := applyMigrations(cfg)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		slog.Warn("db_migrate_retry", "error", err.Error())
		select {
		case <-ctx.Done():
			return fmt.Errorf("migrate: %w", err)
		case <-time.After(retryInterval):
		}
	}
}
