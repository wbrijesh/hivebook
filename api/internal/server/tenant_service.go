package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/protobuf/types/known/timestamppb"

	"api/internal/auth"
	"api/internal/database"
	tenantv1 "api/internal/gen/hivebook/tenant/v1"
	"api/internal/zitadel"
)

// tenantStore is the slice of the data layer this service needs — the tenant
// reads/writes and the audit trail, nothing else. A small consumer-side interface
// (defined here, at the consumer) so the service doesn't depend on the whole
// database.Service surface and is trivially faked in tests.
type tenantStore interface {
	GetOrCreateTenant(ctx context.Context, orgID string) (database.Tenant, error)
	CompleteOnboarding(ctx context.Context, orgID, name, size, region string, useCases []string, useCaseOther string) (database.Tenant, error)
	UpdateTenantProfile(ctx context.Context, orgID, name, size string, useCases []string, useCaseOther string) (database.Tenant, error)
	RecordAuditEvent(ctx context.Context, orgID, actorID, actorEmail, action, target string) error
	ListAuditEvents(ctx context.Context, orgID string, limit, offset int32) ([]database.AuditEvent, int32, error)
	GetFeatureFlags(ctx context.Context, orgID string) (map[string]bool, error)
	SetFeatureFlag(ctx context.Context, orgID, key string, enabled bool) error
	RemoveFeatureFlag(ctx context.Context, orgID, key string) error
}

// memberDirectory is the slice of the identity provider this service reads — the
// workspace's people. Faked in tests; nil/unconfigured in environments without a
// management token (ListMembers then reports configured=false).
type memberDirectory interface {
	Configured() bool
	ListOrgMembers(ctx context.Context, orgID string) ([]zitadel.Member, error)
}

// tenantService implements the Connect TenantService over the data layer and the
// directory. Authentication is handled upstream by the auth interceptor (the
// caller is read from the context); field validation by protovalidate. Stateful
// business rules — region write-once — live here, in application code (design-doc 0006).
type tenantService struct {
	db      tenantStore
	members memberDirectory
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
	s.recordAudit(ctx, id, "tenant.onboarded", m.GetName())
	return connect.NewResponse(&tenantv1.CompleteOnboardingResponse{Tenant: toProtoTenant(t)}), nil
}

// UpdateTenant edits the mutable workspace profile from settings. Field rules
// (name required, lengths) are enforced by protovalidate before this runs; region
// is not a field here at all (write-once, ADR-0014).
func (s *tenantService) UpdateTenant(
	ctx context.Context, req *connect.Request[tenantv1.UpdateTenantRequest],
) (*connect.Response[tenantv1.UpdateTenantResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	m := req.Msg
	t, err := s.db.UpdateTenantProfile(ctx, id.Org, m.GetName(), m.GetSize(), m.GetUseCases(), m.GetUseCaseOther())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("workspace is not set up yet"))
		}
		slog.Error("tenant_update_failed",
			"error", err.Error(), "org", id.Org,
			"request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeInternal, errors.New("update failed"))
	}
	s.recordAudit(ctx, id, "tenant.updated", m.GetName())
	return connect.NewResponse(&tenantv1.UpdateTenantResponse{Tenant: toProtoTenant(t)}), nil
}

// ListMembers returns the workspace's people from the directory (ZITADEL). When
// the directory integration isn't configured it reports configured=false with an
// empty list, so the UI can explain rather than imply the workspace is empty.
func (s *tenantService) ListMembers(
	ctx context.Context, _ *connect.Request[tenantv1.ListMembersRequest],
) (*connect.Response[tenantv1.ListMembersResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	if s.members == nil || !s.members.Configured() {
		return connect.NewResponse(&tenantv1.ListMembersResponse{Configured: false}), nil
	}
	ms, err := s.members.ListOrgMembers(ctx, id.Org)
	if err != nil {
		if errors.Is(err, zitadel.ErrNotConfigured) {
			return connect.NewResponse(&tenantv1.ListMembersResponse{Configured: false}), nil
		}
		slog.Error("members_list_failed",
			"error", err.Error(), "org", id.Org,
			"request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("directory unavailable"))
	}
	out := make([]*tenantv1.Member, 0, len(ms))
	for _, m := range ms {
		out = append(out, &tenantv1.Member{Id: m.ID, Name: m.Name, Email: m.Email, Roles: m.Roles})
	}
	return connect.NewResponse(&tenantv1.ListMembersResponse{Members: out, Configured: true}), nil
}

// ListAuditEvents returns a page of the workspace's audit trail, newest first.
func (s *tenantService) ListAuditEvents(
	ctx context.Context, req *connect.Request[tenantv1.ListAuditEventsRequest],
) (*connect.Response[tenantv1.ListAuditEventsResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	limit := req.Msg.GetLimit()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	events, total, err := s.db.ListAuditEvents(ctx, id.Org, limit, req.Msg.GetOffset())
	if err != nil {
		slog.Error("audit_list_failed",
			"error", err.Error(), "org", id.Org,
			"request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeInternal, errors.New("audit lookup failed"))
	}
	out := make([]*tenantv1.AuditEvent, 0, len(events))
	for _, e := range events {
		out = append(out, &tenantv1.AuditEvent{
			Id:         e.ID,
			OccurredAt: timestamppb.New(e.OccurredAt),
			ActorId:    e.ActorID,
			ActorEmail: e.ActorEmail,
			Action:     e.Action,
			Target:     e.Target,
		})
	}
	return connect.NewResponse(&tenantv1.ListAuditEventsResponse{Events: out, Total: total}), nil
}

// recordAudit appends an audit event, best-effort: a write failure is logged,
// never propagated — an audit hiccup must not fail the user's action (ADR-0010).
func (s *tenantService) recordAudit(ctx context.Context, id auth.Identity, action, target string) {
	if err := s.db.RecordAuditEvent(ctx, id.Org, id.Sub, id.Email, action, target); err != nil {
		slog.Warn("audit_record_failed",
			"action", action, "org", id.Org, "error", err.Error(),
			"request_id", middleware.GetReqID(ctx))
	}
}

// featureFlagDef is one catalog entry: the flag's stable key, user-facing copy,
// and default. The catalog is the source of truth for which flags exist; a
// tenant's overrides (in the database) layer on top.
type featureFlagDef struct {
	Key, Name, Description string
	Default                bool
}

// featureFlagCatalog — add new flags here. Keep keys stable (they're persisted).
var featureFlagCatalog = []featureFlagDef{
	{
		Key:         "github_pat",
		Name:        "GitHub personal access tokens",
		Description: "Connect GitHub with a fine-grained, read-only personal access token — for repositories where you can't install the GitHub App. The App stays the recommended option.",
		Default:     false,
	},
}

func featureFlagByKey(key string) (featureFlagDef, bool) {
	for _, f := range featureFlagCatalog {
		if f.Key == key {
			return f, true
		}
	}
	return featureFlagDef{}, false
}

// featureFlagEnabled resolves a flag's effective value for a tenant: the override
// if present, else the catalog default. Shared by handlers that gate on a flag.
func featureFlagEnabled(overrides map[string]bool, key string) bool {
	if v, ok := overrides[key]; ok {
		return v
	}
	def, _ := featureFlagByKey(key)
	return def.Default
}

// ListFeatureFlags returns the catalog merged with the tenant's overrides.
func (s *tenantService) ListFeatureFlags(
	ctx context.Context, _ *connect.Request[tenantv1.ListFeatureFlagsRequest],
) (*connect.Response[tenantv1.ListFeatureFlagsResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	overrides, err := s.db.GetFeatureFlags(ctx, id.Org)
	if err != nil {
		slog.Error("feature_flags_get_failed",
			"error", err.Error(), "org", id.Org, "request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeInternal, errors.New("feature flags lookup failed"))
	}
	out := make([]*tenantv1.FeatureFlag, 0, len(featureFlagCatalog))
	for _, f := range featureFlagCatalog {
		enabled := f.Default
		if v, present := overrides[f.Key]; present {
			enabled = v
		}
		out = append(out, &tenantv1.FeatureFlag{
			Key: f.Key, Name: f.Name, Description: f.Description, Enabled: enabled,
		})
	}
	return connect.NewResponse(&tenantv1.ListFeatureFlagsResponse{Flags: out}), nil
}

// SetFeatureFlag toggles one known flag for the tenant.
func (s *tenantService) SetFeatureFlag(
	ctx context.Context, req *connect.Request[tenantv1.SetFeatureFlagRequest],
) (*connect.Response[tenantv1.SetFeatureFlagResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	def, known := featureFlagByKey(req.Msg.GetKey())
	if !known {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown feature flag"))
	}
	enabled := req.Msg.GetEnabled()
	// Store only deviations from the catalog default: setting a flag back to its
	// default removes the override (smaller map; global default changes apply).
	var dberr error
	if enabled == def.Default {
		dberr = s.db.RemoveFeatureFlag(ctx, id.Org, def.Key)
	} else {
		dberr = s.db.SetFeatureFlag(ctx, id.Org, def.Key, enabled)
	}
	if dberr != nil {
		slog.Error("feature_flag_set_failed",
			"error", dberr.Error(), "org", id.Org, "key", def.Key,
			"request_id", middleware.GetReqID(ctx))
		return nil, connect.NewError(connect.CodeInternal, errors.New("feature flag update failed"))
	}
	state := "off"
	if enabled {
		state = "on"
	}
	s.recordAudit(ctx, id, "feature_flag.updated", def.Name+" → "+state)
	return connect.NewResponse(&tenantv1.SetFeatureFlagResponse{
		Flag: &tenantv1.FeatureFlag{
			Key: def.Key, Name: def.Name, Description: def.Description, Enabled: enabled,
		},
	}), nil
}
