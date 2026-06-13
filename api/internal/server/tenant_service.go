package server

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5/middleware"

	"api/internal/auth"
	"api/internal/database"
	tenantv1 "api/internal/gen/hivebook/tenant/v1"
)

// tenantStore is the slice of the data layer this service needs — the two tenant
// operations, nothing else. A small consumer-side interface (defined here, at the
// consumer) so the service doesn't depend on the whole database.Service surface
// and is trivially faked in tests.
type tenantStore interface {
	GetOrCreateTenant(ctx context.Context, orgID string) (database.Tenant, error)
	CompleteOnboarding(ctx context.Context, orgID, name, size, region string, useCases []string, useCaseOther string) (database.Tenant, error)
}

// tenantService implements the Connect TenantService over the data layer.
// Authentication is handled upstream by the auth interceptor (the caller is read
// from the context); field validation by protovalidate. Stateful business rules
// — region write-once — live here, in application code (design-doc 0006).
type tenantService struct {
	db tenantStore
}

// toProtoTenant maps the data-layer tenant to the wire type. The optional proto
// fields and the data layer's *string nullables line up one-to-one.
func toProtoTenant(t database.Tenant) *tenantv1.Tenant {
	return &tenantv1.Tenant{
		Id:           t.ID,
		Name:         t.Name,
		Size:         t.Size,
		Region:       t.Region,
		UseCases:     t.UseCases,
		UseCaseOther: t.UseCaseOther,
		Onboarded:    t.OnboardedAt != nil,
	}
}

// GetSession returns the caller's identity and tenant — both resolved
// server-side, never trusted from the client — lazily creating the tenant row on
// first sight.
func (s *tenantService) GetSession(
	ctx context.Context, _ *connect.Request[tenantv1.GetSessionRequest],
) (*connect.Response[tenantv1.GetSessionResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	t, err := s.db.GetOrCreateTenant(ctx, id.Org)
	if err != nil {
		slog.Error("tenant_get_failed",
			"error", err.Error(), "org", id.Org,
			"request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeInternal, errors.New("tenant lookup failed"))
	}
	return connect.NewResponse(&tenantv1.GetSessionResponse{
		User:   &tenantv1.User{Id: id.Sub, Name: id.Name, Email: id.Email},
		Tenant: toProtoTenant(t),
	}), nil
}

// CompleteOnboarding records the onboarding answers and marks the tenant
// onboarded. Field rules (region required, lengths) are enforced by protovalidate
// before this runs; region write-once is enforced atomically in the database and
// surfaced here as CodeFailedPrecondition.
//
// RBAC: v0.1 treats any signed-in member of the org as able to onboard it (the
// "signed-in user is admin" assumption, design-doc 0005). Gate on an owner/admin
// role once the role model lands (enterprise-controls).
func (s *tenantService) CompleteOnboarding(
	ctx context.Context, req *connect.Request[tenantv1.CompleteOnboardingRequest],
) (*connect.Response[tenantv1.CompleteOnboardingResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	m := req.Msg
	t, err := s.db.CompleteOnboarding(ctx, id.Org,
		m.GetName(), m.GetSize(), m.GetRegion(), m.GetUseCases(), m.GetUseCaseOther())
	if err != nil {
		if errors.Is(err, database.ErrRegionImmutable) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("region cannot be changed after setup"))
		}
		slog.Error("tenant_onboarding_failed",
			"error", err.Error(), "org", id.Org,
			"request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeInternal, errors.New("onboarding failed"))
	}

	// Onboarding completion is a privileged write — audit it, correlated to the
	// tenant and the acting user (design-doc 0005).
	slog.Info("tenant_onboarded",
		"tenant_id", t.ID, "org", id.Org, "user_id", id.Sub, "region", m.GetRegion(),
		"request_id", middleware.GetReqID(ctx))
	return connect.NewResponse(&tenantv1.CompleteOnboardingResponse{Tenant: toProtoTenant(t)}), nil
}
