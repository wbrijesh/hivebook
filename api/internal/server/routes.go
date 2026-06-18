package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"api/internal/gen/hivebook/integration/v1/integrationv1connect"
	"api/internal/gen/hivebook/tenant/v1/tenantv1connect"
)

func (s *Server) RegisterRoutes() http.Handler {
	// Connection-pool gauges, scraped at /metrics.
	prometheus.MustRegister(newDBStatsCollector(s.db.Stats))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(structuredLogger)
	r.Use(metricsMiddleware)
	r.Use(cors.Handler(corsOptions()))

	// The Connect API. Interceptors run outermost-first: sanitize (scrubs internal
	// detail from outgoing errors), then metrics (times the whole RPC, records every
	// outcome — sanitize preserves the code, so counts stay correct), then auth
	// (caller resolution), then protovalidate (field rules). The service mounts as a
	// plain http.Handler under its RPC path prefix; the middleware above keeps
	// wrapping it.
	interceptors := connect.WithInterceptors(
		newSanitizeInterceptor(),
		newMetricsInterceptor(),
		newAuthInterceptor(s.auth),
		s.validate,
	)
	r.Mount(tenantv1connect.NewTenantServiceHandler(
		&tenantService{db: s.db, members: s.zitadel}, interceptors))

	// The user-facing integrations surface — proxies to the internal integrations
	// service, attaching the caller's tenant (design-doc 0009).
	r.Mount(integrationv1connect.NewIntegrationServiceHandler(
		&integrationService{
			client:       s.integration,
			streamClient: s.integrationStream,
			internal:     s.integrationInternal,
			db:           s.db,
		}, interceptors))

	// gRPC server reflection and health, so grpcurl / buf curl / agent tooling can
	// introspect and probe the API (design-doc 0006). Both reflection versions are
	// registered for client compatibility; health mirrors the DB readiness.
	reflector := grpcreflect.NewStaticReflector(tenantv1connect.TenantServiceName)
	r.Mount(grpcreflect.NewHandlerV1(reflector))
	r.Mount(grpcreflect.NewHandlerV1Alpha(reflector))
	r.Mount(grpchealth.NewHandler(dbHealthChecker{db: s.db}))

	// Public, non-RPC routes — operational surfaces, the telemetry beacon, and the
	// connector OAuth callback (the provider redirects the browser here; it trusts
	// the signed state, not a session — design-doc 0009).
	r.Get("/", s.rootHandler)
	r.Get("/health", s.healthHandler)
	r.Handle("/metrics", promhttp.Handler())
	r.Post("/events", s.eventsHandler)
	r.Get("/connectors/{connector}/callback", s.connectorCallback)

	// h2c so the same listener serves HTTP/2 cleartext (real gRPC) and HTTP/1.1
	// (Connect, gRPC-Web, kubelet probes) — no TLS at the pod; Traefik terminates it.
	// The http.Server's Read/WriteTimeouts don't apply per-stream on HTTP/2, so set
	// an idle timeout and a concurrent-stream cap here (slow-loris / fan-out posture)
	// in addition to whatever Traefik enforces upstream.
	return h2c.NewHandler(r, &http2.Server{
		IdleTimeout:          time.Minute,
		MaxConcurrentStreams: 250,
	})
}

// dbHealthChecker answers the gRPC health protocol from the database readiness,
// so a gRPC probe agrees with the HTTP /health endpoint.
type dbHealthChecker struct {
	db interface{ Health() map[string]string }
}

func (c dbHealthChecker) Check(context.Context, *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	status := grpchealth.StatusServing
	if c.db.Health()["status"] != "up" {
		status = grpchealth.StatusNotServing
	}
	return &grpchealth.CheckResponse{Status: status}, nil
}

// corsOptions allows the configured browser origins to call the Connect API.
// Origins are explicit (HIVEBOOK_CORS_ORIGINS, comma-separated) — never a
// wildcard with credentials — and the allowed/exposed headers come from the
// Connect helper so all three protocols work cross-origin.
//
// The Connect helper covers only protocol headers, not Authorization — we send
// the OIDC bearer token in it, so it must be allowed explicitly or the browser's
// preflight for an authenticated request is rejected and the call never lands.
func corsOptions() cors.Options {
	return cors.Options{
		AllowedOrigins:   splitOrigins(os.Getenv("HIVEBOOK_CORS_ORIGINS")),
		AllowedMethods:   connectcors.AllowedMethods(),
		AllowedHeaders:   append(connectcors.AllowedHeaders(), "Authorization"),
		ExposedHeaders:   connectcors.ExposedHeaders(),
		AllowCredentials: true,
		MaxAge:           7200, // seconds; cap preflight churn
	}
}

func splitOrigins(s string) []string {
	var out []string
	for _, o := range strings.Split(s, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func (s *Server) rootHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "hivebook-api"})
}

// healthHandler returns the DB health and a 503 when it's down, so readiness and
// liveness probes mean something.
func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	h := s.db.Health()
	status := http.StatusOK
	if h["status"] != "up" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, h)
}

// writeJSON is the one path for JSON success responses on the plain (non-Connect)
// handlers — sets the content type, then encodes. Connect handlers return typed
// messages and never touch this.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
