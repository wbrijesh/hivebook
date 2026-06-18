package server

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"integrations/internal/database"
)

var testCfg database.Config

func TestMain(m *testing.M) {
	c, err := postgres.Run(
		context.Background(),
		"postgres:17-alpine",
		postgres.WithDatabase("database"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		log.Fatalf("start postgres: %v", err)
	}
	host, _ := c.Host(context.Background())
	port, _ := c.MappedPort(context.Background(), "5432/tcp")
	testCfg = database.Config{Host: host, Port: port.Port(), User: "user", Password: "password", Database: "database"}

	code := m.Run()
	_ = c.Terminate(context.Background())
	os.Exit(code)
}

// newStore opens a store against the TestMain container (migrations run on New).
func newStore(t *testing.T) *database.Store {
	t.Helper()
	s, err := database.New(testCfg)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
