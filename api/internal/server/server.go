// Package server is the HTTP layer: the chi router and middleware, the Connect
// API (design-doc 0006), and the plain operational handlers, over the auth and
// database packages.
package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"api/internal/auth"
	"api/internal/database"
)

type Server struct {
	port int

	db       database.Service
	auth     authenticator
	validate connect.Interceptor // protovalidate, built once
}

// NewServer wires the API: database, auth, validation, and routes. Returns an
// error rather than fatally exiting so the caller (main) owns process lifecycle.
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
		// protovalidate runs the CEL field constraints declared on the proto,
		// before any handler.
		validate: validate.NewInterceptor(),
	}

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", srv.port),
		Handler:      srv.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}, nil
}
