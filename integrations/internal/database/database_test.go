package database

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testCfg points at the container started in TestMain; tests pass it to New.
var testCfg Config

func mustStartPostgresContainer() (func(context.Context, ...testcontainers.TerminateOption) error, error) {
	dbName, dbUser, dbPwd := "database", "user", "password"

	c, err := postgres.Run(
		context.Background(),
		"postgres:17-alpine", // pinned, matches the production StatefulSet
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPwd),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		return nil, err
	}

	host, err := c.Host(context.Background())
	if err != nil {
		return c.Terminate, err
	}
	port, err := c.MappedPort(context.Background(), "5432/tcp")
	if err != nil {
		return c.Terminate, err
	}

	testCfg = Config{
		Host:     host,
		Port:     port.Port(),
		User:     dbUser,
		Password: dbPwd,
		Database: dbName,
	}
	return c.Terminate, nil
}

func TestMain(m *testing.M) {
	teardown, err := mustStartPostgresContainer()
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	m.Run()

	if teardown != nil {
		if err := teardown(context.Background()); err != nil {
			log.Fatalf("could not teardown postgres container: %v", err)
		}
	}
}
