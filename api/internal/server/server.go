package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

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

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	NewServer := &Server{
		port: port,

		db: database.New(),
		auth: auth.New(
			os.Getenv("HIVEBOOK_OIDC_ISSUER"),
			os.Getenv("HIVEBOOK_OIDC_AUDIENCE"),
			os.Getenv("HIVEBOOK_OIDC_CA_FILE"),
		),
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
