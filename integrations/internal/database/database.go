// Package database is the integrations data layer: a typed query store (sqlc)
// over Postgres, with schema migrations applied on startup (golang-migrate). The
// integrations service owns its own database; the API never reads it (design-doc
// 0009).
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"integrations/internal/database/gen"
)

// Connection-pool tuning and the startup migration window. Sized for the
// service's modest concurrency; maxIdle == maxOpen keeps the pool warm.
const (
	maxOpenConns    = 10
	maxIdleConns    = 10
	connMaxLifetime = 5 * time.Minute
	connMaxIdleTime = 5 * time.Minute
	migrateTimeout  = 90 * time.Second
)

// Config is the database connection configuration.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Schema   string // defaults to "public"
	SSLMode  string // defaults to "disable"
}

// ConfigFromEnv reads the HIVEBOOK_DB_* environment. The deployment points
// HIVEBOOK_DB_DATABASE at the integrations database on the shared instance.
func ConfigFromEnv() Config {
	return Config{
		Host:     os.Getenv("HIVEBOOK_DB_HOST"),
		Port:     os.Getenv("HIVEBOOK_DB_PORT"),
		User:     os.Getenv("HIVEBOOK_DB_USERNAME"),
		Password: os.Getenv("HIVEBOOK_DB_PASSWORD"),
		Database: os.Getenv("HIVEBOOK_DB_DATABASE"),
		Schema:   os.Getenv("HIVEBOOK_DB_SCHEMA"),
		SSLMode:  os.Getenv("HIVEBOOK_DB_SSLMODE"),
	}
}

// DSN builds a libpq URL with the password percent-escaped via url.UserPassword.
func (c Config) DSN() string {
	schema := c.Schema
	if schema == "" {
		schema = "public"
	}
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + c.Database,
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

// Store is the data layer: the sqlc query set plus pool lifecycle. Domain logic
// (token encryption, proto mapping) lives in the callers, not here — the schema
// is plain CRUD.
type Store struct {
	db *sql.DB
	Q  *gen.Queries
}

// New opens the pool, tunes it, applies migrations (retrying within a bounded
// window since Postgres may not be up yet), and returns the store.
func New(cfg Config) (*Store, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()
	if err := migrateWithRetry(ctx, cfg); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db, Q: gen.New(db)}, nil
}

// Health pings the database and reports pool stats. "status" is "up" or "down".
func (s *Store) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		slog.Error("db_health_down", "error", err.Error())
		return map[string]string{"status": "down", "error": err.Error()}
	}
	d := s.db.Stats()
	return map[string]string{
		"status":           "up",
		"open_connections": strconv.Itoa(d.OpenConnections),
		"in_use":           strconv.Itoa(d.InUse),
		"idle":             strconv.Itoa(d.Idle),
	}
}

// Stats returns the sql.DB connection-pool statistics.
func (s *Store) Stats() sql.DBStats { return s.db.Stats() }

// SQLDB exposes the underlying pool for the River job queue (insert-only client +
// River migrations), which lives alongside the sqlc query set (design-doc 0010).
func (s *Store) SQLDB() *sql.DB { return s.db }

// Close terminates the database connection.
func (s *Store) Close() error { return s.db.Close() }
