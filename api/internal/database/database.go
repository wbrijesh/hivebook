// Package database is the data layer: a typed query store (sqlc, ADR-0018) over
// Postgres, with schema migrations applied on startup (golang-migrate, ADR-0017).
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

	"api/internal/database/gen"
)

// Connection-pool tuning and the startup migration window (constants-first, ADR-0011).
// maxIdle == maxOpen keeps the pool warm — no connect/teardown churn under bursty
// load — at the cost of holding up to maxOpen idle connections; sized for the API's
// modest concurrency, with lifetime/idle caps so stale connections still recycle.
const (
	maxOpenConns    = 25
	maxIdleConns    = 25
	connMaxLifetime = 5 * time.Minute
	connMaxIdleTime = 5 * time.Minute
	migrateTimeout  = 90 * time.Second
)

// Service is the data layer the rest of the app depends on.
type Service interface {
	// Health reports connectivity and pool stats. "status" is "up" or "down".
	Health() map[string]string

	// Stats returns the connection-pool statistics, exported as Prometheus gauges
	// (see internal/server/dbstats.go).
	Stats() sql.DBStats

	// GetOrCreateTenant returns the tenant for a ZITADEL org id, creating an empty
	// (un-onboarded) row the first time the org is seen.
	GetOrCreateTenant(ctx context.Context, orgID string) (Tenant, error)

	// CompleteOnboarding records the onboarding answers and marks the tenant
	// onboarded. Idempotent; the storage region is write-once.
	CompleteOnboarding(ctx context.Context, orgID, name, size, region string, useCases []string, useCaseOther string) (Tenant, error)

	// Close terminates the database connection.
	Close() error
}

// Config is the database connection configuration — no package globals.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Schema   string // defaults to "public"
	SSLMode  string // defaults to "disable"
}

// ConfigFromEnv reads the HIVEBOOK_DB_* environment.
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

// dsn builds a libpq URL with the password percent-escaped via url.UserPassword.
func (c Config) dsn() string {
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

type service struct {
	db *sql.DB
	q  *gen.Queries
}

// New opens the pool, tunes it, applies migrations (retrying within a bounded
// window since Postgres may not be up yet), and returns the service.
func New(cfg Config) (Service, error) {
	db, err := sql.Open("pgx", cfg.dsn())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	// Apply migrations on a separate short-lived connection (not the app pool),
	// retrying within a bounded window since Postgres may not be up yet.
	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()
	if err := migrateWithRetry(ctx, cfg); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &service{db: db, q: gen.New(db)}, nil
}

// Health pings the database and reports pool stats. "status" is "up" or "down";
// the /health handler turns "down" into a 503.
func (s *service) Health() map[string]string {
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
func (s *service) Stats() sql.DBStats {
	return s.db.Stats()
}

// Close terminates the database connection.
func (s *service) Close() error {
	return s.db.Close()
}
