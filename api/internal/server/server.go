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
	"api/internal/gen/hivebook/integration/v1/integrationv1connect"
	"api/internal/zitadel"
)

type Server struct {
	port int

	db       database.Service
	auth     authenticator
	zitadel  *zitadel.Client     // directory reads (members); optional, may be unconfigured
	validate connect.Interceptor // protovalidate, built once

	// Clients for the internal integrations service (design-doc 0009). The API
	// proxies the user-facing IntegrationService and calls the internal
	// CompleteConnection from the OAuth callback.
	integration         integrationv1connect.IntegrationServiceClient
	integrationStream   integrationv1connect.IntegrationServiceClient // no client timeout — for WatchConnections
	integrationInternal integrationv1connect.IntegrationInternalServiceClient
	webBaseURL          string // where the OAuth callback redirects the browser back to
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

	integrationsURL := os.Getenv("HIVEBOOK_INTEGRATIONS_URL")
	igHTTP := &http.Client{Timeout: 30 * time.Second}
	// A separate client with no timeout for the WatchConnections server stream —
	// a finite client timeout would sever the long-lived stream.
	igStreamHTTP := &http.Client{}

	srv := &Server{
		port: port,
		db:   db,
		auth: auth.New(
			os.Getenv("HIVEBOOK_OIDC_ISSUER"),
			os.Getenv("HIVEBOOK_OIDC_AUDIENCE"),
			os.Getenv("HIVEBOOK_OIDC_CA_FILE"),
		),
		// Directory client for the Members surface. Shares the issuer + mkcert CA
		// with auth; the token is a service-account PAT, optional so the API runs
		// before it's provisioned (Members then reports "not configured").
		zitadel: zitadel.New(
			os.Getenv("HIVEBOOK_OIDC_ISSUER"),
			os.Getenv("HIVEBOOK_ZITADEL_MGMT_TOKEN"),
			os.Getenv("HIVEBOOK_OIDC_CA_FILE"),
		),
		// protovalidate runs the CEL field constraints declared on the proto,
		// before any handler.
		validate:            validate.NewInterceptor(),
		integration:         integrationv1connect.NewIntegrationServiceClient(igHTTP, integrationsURL),
		integrationStream:   integrationv1connect.NewIntegrationServiceClient(igStreamHTTP, integrationsURL),
		integrationInternal: integrationv1connect.NewIntegrationInternalServiceClient(igHTTP, integrationsURL),
		webBaseURL:          os.Getenv("HIVEBOOK_WEB_BASE_URL"),
	}

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", srv.port),
		Handler: srv.RegisterRoutes(),
		// No WriteTimeout: WatchConnections is a long-lived server stream that a
		// finite write deadline would cut. ReadHeaderTimeout keeps slow-loris
		// protection; IdleTimeout reaps idle keep-alives.
		IdleTimeout:       time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
	}, nil
}
