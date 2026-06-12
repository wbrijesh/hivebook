package database

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/golang-migrate/migrate/v4"
)

func connStrTo(name string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port, name)
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var ok bool
	if err := db.QueryRow(`SELECT to_regclass('public.' || $1) IS NOT NULL`, name).Scan(&ok); err != nil {
		t.Fatalf("table check: %v", err)
	}
	return ok
}

func columnExists(t *testing.T, db *sql.DB, table, col string) bool {
	t.Helper()
	var ok bool
	if err := db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name=$2)`,
		table, col).Scan(&ok); err != nil {
		t.Fatalf("column check: %v", err)
	}
	return ok
}

// TestMigrations_UpDownUp runs the migrations against a throwaway database in the
// shared container — proving up applies the full schema, up is idempotent, and the
// down files are valid (down then up again).
func TestMigrations_UpDownUp(t *testing.T) {
	admin, err := sql.Open("pgx", connStrTo(database))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.Exec(`DROP DATABASE IF EXISTS migtest`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`CREATE DATABASE migtest`); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("pgx", connStrTo("migtest"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m, err := newMigrator(db)
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if !tableExists(t, db, "tenants") {
		t.Fatal("tenants table missing after up")
	}
	if !columnExists(t, db, "tenants", "use_case_other") {
		t.Fatal("use_case_other column missing after up")
	}

	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("expected ErrNoChange on second up, got %v", err)
	}

	if err := m.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if tableExists(t, db, "tenants") {
		t.Fatal("tenants table should be dropped after down")
	}

	if err := m.Up(); err != nil {
		t.Fatalf("re-up: %v", err)
	}
	if !tableExists(t, db, "tenants") {
		t.Fatal("tenants table missing after re-up")
	}
}
