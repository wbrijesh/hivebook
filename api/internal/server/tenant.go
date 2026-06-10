package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"api/internal/auth"
	"api/internal/database"
)

// writeJSON is the one path for JSON success responses — always sets the content
// type, then encodes (standards/go/errors).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type tenantResponse struct {
	ID           string   `json:"id"`
	Name         *string  `json:"name"`
	Size         *string  `json:"size"`
	Region       *string  `json:"region"`
	UseCases     []string `json:"useCases"`
	UseCaseOther *string  `json:"useCaseOther"`
	Onboarded    bool     `json:"onboarded"`
}

func toTenantResponse(t database.Tenant) tenantResponse {
	return tenantResponse{
		ID:           t.ID,
		Name:         t.Name,
		Size:         t.Size,
		Region:       t.Region,
		UseCases:     t.UseCases,
		UseCaseOther: t.UseCaseOther,
		Onboarded:    t.OnboardedAt != nil,
	}
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// callerFromRequest resolves the authenticated caller — their identity and the
// tenant (ZITADEL org) that owns them — for the request, writing a 422 if it
// can't be determined. The single caller-resolution path for protected handlers.
func (s *Server) callerFromRequest(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	id, err := s.auth.Identity(r.Context(), r)
	if err != nil || id.Org == "" {
		slog.Warn("tenant: could not resolve caller",
			"error", errStr(err),
			"request_id", middleware.GetReqID(r.Context()))
		http.Error(w, "could not resolve identity", http.StatusUnprocessableEntity)
		return auth.Identity{}, false
	}
	return id, true
}

// tenantForCaller loads — lazily creating on first sight — the tenant for an
// org, writing a 500 on failure. Shared by the handlers that return tenant state.
func (s *Server) tenantForCaller(w http.ResponseWriter, r *http.Request, org string) (database.Tenant, bool) {
	t, err := s.db.GetOrCreateTenant(r.Context(), org)
	if err != nil {
		slog.Error("tenant: get/create failed",
			"error", err.Error(),
			"request_id", middleware.GetReqID(r.Context()))
		http.Error(w, "tenant lookup failed", http.StatusInternalServerError)
		return database.Tenant{}, false
	}
	return t, true
}

// tenantHandler returns the caller's tenant and onboarding status, creating the
// row lazily.
func (s *Server) tenantHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := s.callerFromRequest(w, r)
	if !ok {
		return
	}
	t, ok := s.tenantForCaller(w, r, id.Org)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toTenantResponse(t))
}

type onboardingRequest struct {
	Name         string   `json:"name"`
	Size         string   `json:"size"`
	Region       string   `json:"region"`
	UseCases     []string `json:"useCases"`
	UseCaseOther string   `json:"useCaseOther"`
}

// onboardingHandler records the onboarding answers and marks the tenant
// onboarded. Idempotent; region is write-once.
//
// RBAC: v0.1 treats any signed-in member of the org as able to onboard it (the
// "signed-in user is admin" assumption in design-doc 0005). Gate this on an
// owner/admin role once the role model lands (enterprise-controls).
func (s *Server) onboardingHandler(w http.ResponseWriter, r *http.Request) {
	id, ok := s.callerFromRequest(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var req onboardingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Region == "" {
		http.Error(w, "region is required", http.StatusBadRequest)
		return
	}

	t, err := s.db.CompleteOnboarding(r.Context(), id.Org, req.Name, req.Size, req.Region, req.UseCases, req.UseCaseOther)
	if err != nil {
		if errors.Is(err, database.ErrRegionImmutable) {
			http.Error(w, "region cannot be changed after setup", http.StatusConflict)
			return
		}
		slog.Error("tenant: onboarding failed",
			"error", err.Error(),
			"request_id", middleware.GetReqID(r.Context()))
		http.Error(w, "onboarding failed", http.StatusInternalServerError)
		return
	}

	// Onboarding completion is a privileged write — audit it, correlated to the
	// tenant and the acting user (design-doc 0005).
	slog.Info("tenant_onboarded",
		"tenant_id", t.ID,
		"org", id.Org,
		"user_id", id.Sub,
		"region", req.Region,
		"request_id", middleware.GetReqID(r.Context()))
	writeJSON(w, http.StatusOK, toTenantResponse(t))
}
