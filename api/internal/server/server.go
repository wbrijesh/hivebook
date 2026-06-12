// Package server is the HTTP layer: the chi router, middleware, and handlers
// that expose the API over the auth and database packages.
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"api/internal/auth"
	"api/internal/database"
)

// authenticator is the slice of the auth layer the server depends on: protecting
// routes and resolving the caller's identity. A small interface so handlers can
// be tested with a stub. *auth.Authenticator satisfies it.
type authenticator interface {
	Middleware(http.Handler) http.Handler
	Identity(context.Context, *http.Request) (auth.Identity, error)
}

type Server struct {
	port int

	db   database.Service
	auth authenticator
}

// NewServer wires the API: database, auth, and routes. Returns an error rather
// than fatally exiting so the caller (main) owns process lifecycle.
func NewServer() (*http.Server, error) {
	port := 8080
	if v := os.Getenv("HIVEBOOK_API_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid HIVEBOOK_API_PORT %q: %w", v, err)
		}
		port = p
	}

	db, err := database.New(database.ConfigFromEnv())
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	srv := &Server{
		port: port,
		db:   db,
		auth: auth.New(
			os.Getenv("HIVEBOOK_OIDC_ISSUER"),
			os.Getenv("HIVEBOOK_OIDC_AUDIENCE"),
			os.Getenv("HIVEBOOK_OIDC_CA_FILE"),
		),
	}

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", srv.port),
		Handler:      srv.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, nil
}
