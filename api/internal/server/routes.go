package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) RegisterRoutes() http.Handler {
	// Connection-pool gauges, scraped at /metrics.
	prometheus.MustRegister(newDBStatsCollector(s.db.Stats))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(structuredLogger)
	r.Use(metricsMiddleware)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes.
	r.Get("/", s.rootHandler)
	r.Get("/health", s.healthHandler)
	r.Handle("/metrics", promhttp.Handler())
	r.Post("/events", s.eventsHandler)

	// Protected routes — require a valid ZITADEL access token.
	r.Group(func(pr chi.Router) {
		pr.Use(s.auth.Middleware)
		pr.Get("/api/me", s.meHandler)
		pr.Get("/api/tenant", s.tenantHandler)
		pr.Post("/api/tenant/onboarding", s.onboardingHandler)
	})

	return r
}

func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"service": "hivebook-api"})
}

// healthHandler returns the DB health and a 503 when it's down, so readiness and
// liveness probes mean something.
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	h := s.db.Health()
	status := http.StatusOK
	if h["status"] != "up" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, h)
}

type meUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type meResponse struct {
	User   meUser         `json:"user"`
	Tenant tenantResponse `json:"tenant"`
}

// meHandler returns the authenticated caller's identity and tenant — both
// resolved server-side from the token/userinfo and the database, never trusted
// from the client (design-doc 0005). This is the chrome's single source for who
// the user is and which workspace they're in.
func (s *Server) meHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := s.callerFromRequest(w, r)
	if !ok {
		return
	}
	t, ok := s.tenantForCaller(w, r, id.Org)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, meResponse{
		User:   meUser{ID: id.Sub, Name: id.Name, Email: id.Email},
		Tenant: toTenantResponse(t),
	})
}
