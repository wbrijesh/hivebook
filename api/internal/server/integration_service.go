package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"api/internal/auth"
	"api/internal/database"
	integrationv1 "api/internal/gen/hivebook/integration/v1"
	"api/internal/gen/hivebook/integration/v1/integrationv1connect"
)

// Internal-hop headers the integrations service trusts (design-doc 0009).
const (
	headerTenant = "Hivebook-Tenant-Id"
	headerRegion = "Hivebook-Region"
)

// integrationService is the API's user-facing IntegrationService: it proxies to
// the internal integrations service, attaching the caller's tenant (resolved from
// the JWT) as a header. The frontend talks only to this.
type integrationService struct {
	client       integrationv1connect.IntegrationServiceClient
	streamClient integrationv1connect.IntegrationServiceClient // no-timeout, for WatchConnections
	internal     integrationv1connect.IntegrationInternalServiceClient
	db           database.Service
}

// ListConnectors is tenant-independent — the catalog of source types.
func (s *integrationService) ListConnectors(ctx context.Context, _ *connect.Request[integrationv1.ListConnectorsRequest]) (*connect.Response[integrationv1.ListConnectorsResponse], error) {
	res, err := s.client.ListConnectors(ctx, connect.NewRequest(&integrationv1.ListConnectorsRequest{}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) ListConnections(ctx context.Context, _ *connect.Request[integrationv1.ListConnectionsRequest]) (*connect.Response[integrationv1.ListConnectionsResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.ListConnectionsRequest{})
	if err != nil {
		return nil, err
	}
	res, err := s.client.ListConnections(ctx, out)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) GetAuthorizeUrl(ctx context.Context, req *connect.Request[integrationv1.GetAuthorizeUrlRequest]) (*connect.Response[integrationv1.GetAuthorizeUrlResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	// The connection's residency region comes from the tenant record; it is signed
	// into the OAuth state so CompleteConnection can stamp it.
	t, err := s.db.GetOrCreateTenant(ctx, id.Org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("tenant lookup failed"))
	}
	out := connect.NewRequest(&integrationv1.GetAuthorizeUrlRequest{ConnectorId: req.Msg.GetConnectorId()})
	out.Header().Set(headerTenant, id.Org)
	if t.Region != nil {
		out.Header().Set(headerRegion, *t.Region)
	}
	res, err := s.client.GetAuthorizeUrl(ctx, out)
	if err != nil {
		return nil, err
	}
	// The user authorizing a connector is a workspace action — trail it. The OAuth
	// dance may still fail downstream; this records the intent, with the real actor.
	recordSourceAudit(ctx, s.db, "source.connect_started", req.Msg.GetConnectorId())
	return connect.NewResponse(res.Msg), nil
}

// ConnectWithToken connects a token-auth source (e.g. github_pat). It enforces the
// connector's feature flag (defense in depth beyond the UI), then hands the token
// to the integrations service over the internal hop with the tenant + region.
func (s *integrationService) ConnectWithToken(ctx context.Context, req *connect.Request[integrationv1.ConnectWithTokenRequest]) (*connect.Response[integrationv1.ConnectWithTokenResponse], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	connectorID := req.Msg.GetConnectorId()
	if connectorID == "github_pat" {
		flags, err := s.db.GetFeatureFlags(ctx, id.Org)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("flag lookup failed"))
		}
		if !featureFlagEnabled(flags, "github_pat") {
			return nil, connect.NewError(connect.CodePermissionDenied,
				errors.New("personal access tokens aren't enabled for this workspace"))
		}
	}
	// The connection's residency region comes from the tenant record.
	t, err := s.db.GetOrCreateTenant(ctx, id.Org)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("tenant lookup failed"))
	}
	out := connect.NewRequest(&integrationv1.CompleteTokenConnectionRequest{
		ConnectorId: connectorID, Token: req.Msg.GetToken(), Name: req.Msg.GetName(),
	})
	out.Header().Set(headerTenant, id.Org)
	if t.Region != nil {
		out.Header().Set(headerRegion, *t.Region)
	}
	res, err := s.internal.CompleteTokenConnection(ctx, out)
	if err != nil {
		return nil, err
	}
	recordSourceAudit(ctx, s.db, "source.connected", connectorID)
	return connect.NewResponse(&integrationv1.ConnectWithTokenResponse{Connection: res.Msg.GetConnection()}), nil
}

func (s *integrationService) TriggerSync(ctx context.Context, req *connect.Request[integrationv1.TriggerSyncRequest]) (*connect.Response[integrationv1.TriggerSyncResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.TriggerSyncRequest{ConnectionId: req.Msg.GetConnectionId()})
	if err != nil {
		return nil, err
	}
	res, err := s.client.TriggerSync(ctx, out)
	if err != nil {
		return nil, err
	}
	recordSourceAudit(ctx, s.db, "source.sync_triggered", req.Msg.GetConnectionId())
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) StopSync(ctx context.Context, req *connect.Request[integrationv1.StopSyncRequest]) (*connect.Response[integrationv1.StopSyncResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.StopSyncRequest{ConnectionId: req.Msg.GetConnectionId()})
	if err != nil {
		return nil, err
	}
	res, err := s.client.StopSync(ctx, out)
	if err != nil {
		return nil, err
	}
	recordSourceAudit(ctx, s.db, "source.sync_stopped", req.Msg.GetConnectionId())
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) Disconnect(ctx context.Context, req *connect.Request[integrationv1.DisconnectRequest]) (*connect.Response[integrationv1.DisconnectResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.DisconnectRequest{ConnectionId: req.Msg.GetConnectionId()})
	if err != nil {
		return nil, err
	}
	res, err := s.client.Disconnect(ctx, out)
	if err != nil {
		return nil, err
	}
	recordSourceAudit(ctx, s.db, "source.disconnected", req.Msg.GetConnectionId())
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) ListArtifacts(ctx context.Context, req *connect.Request[integrationv1.ListArtifactsRequest]) (*connect.Response[integrationv1.ListArtifactsResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.ListArtifactsRequest{
		Limit: req.Msg.GetLimit(), Offset: req.Msg.GetOffset(), ConnectionId: req.Msg.GetConnectionId(),
	})
	if err != nil {
		return nil, err
	}
	res, err := s.client.ListArtifacts(ctx, out)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) GetArtifactContent(ctx context.Context, req *connect.Request[integrationv1.GetArtifactContentRequest]) (*connect.Response[integrationv1.GetArtifactContentResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.GetArtifactContentRequest{Id: req.Msg.GetId()})
	if err != nil {
		return nil, err
	}
	res, err := s.client.GetArtifactContent(ctx, out)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) ListProjects(ctx context.Context, req *connect.Request[integrationv1.ListProjectsRequest]) (*connect.Response[integrationv1.ListProjectsResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.ListProjectsRequest{ConnectionId: req.Msg.GetConnectionId()})
	if err != nil {
		return nil, err
	}
	res, err := s.client.ListProjects(ctx, out)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) SetProjectSelection(ctx context.Context, req *connect.Request[integrationv1.SetProjectSelectionRequest]) (*connect.Response[integrationv1.SetProjectSelectionResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.SetProjectSelectionRequest{
		ConnectionId: req.Msg.GetConnectionId(), SelectedIds: req.Msg.GetSelectedIds(),
	})
	if err != nil {
		return nil, err
	}
	res, err := s.client.SetProjectSelection(ctx, out)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

func (s *integrationService) UpdateSyncSchedule(ctx context.Context, req *connect.Request[integrationv1.UpdateSyncScheduleRequest]) (*connect.Response[integrationv1.UpdateSyncScheduleResponse], error) {
	out, err := tenantRequest(ctx, &integrationv1.UpdateSyncScheduleRequest{
		ConnectionId: req.Msg.GetConnectionId(), Enabled: req.Msg.GetEnabled(), IntervalSeconds: req.Msg.GetIntervalSeconds(),
	})
	if err != nil {
		return nil, err
	}
	res, err := s.client.UpdateSyncSchedule(ctx, out)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res.Msg), nil
}

// WatchProjects proxies the per-project status stream (no-timeout client).
func (s *integrationService) WatchProjects(ctx context.Context, req *connect.Request[integrationv1.WatchProjectsRequest], stream *connect.ServerStream[integrationv1.WatchProjectsResponse]) error {
	out, err := tenantRequest(ctx, &integrationv1.WatchProjectsRequest{ConnectionId: req.Msg.GetConnectionId()})
	if err != nil {
		return err
	}
	upstream, err := s.streamClient.WatchProjects(ctx, out)
	if err != nil {
		return err
	}
	defer upstream.Close()
	for upstream.Receive() {
		if err := stream.Send(upstream.Msg()); err != nil {
			return err
		}
	}
	return upstream.Err()
}

// WatchConnections proxies the integrations service's connection stream to the
// frontend, relaying each snapshot. It uses the no-timeout streaming client so
// the long-lived stream isn't severed (server.go).
func (s *integrationService) WatchConnections(ctx context.Context, req *connect.Request[integrationv1.WatchConnectionsRequest], stream *connect.ServerStream[integrationv1.WatchConnectionsResponse]) error {
	out, err := tenantRequest(ctx, &integrationv1.WatchConnectionsRequest{})
	if err != nil {
		return err
	}
	upstream, err := s.streamClient.WatchConnections(ctx, out)
	if err != nil {
		return err
	}
	defer upstream.Close()
	for upstream.Receive() {
		if err := stream.Send(upstream.Msg()); err != nil {
			return err // client went away
		}
	}
	return upstream.Err()
}

// recordSourceAudit trails a source action, best-effort: it reads the caller from
// context and appends an audit event, logging (never failing the action) on error
// or when there's no identity (ADR-0010).
func recordSourceAudit(ctx context.Context, db database.Service, action, target string) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return
	}
	if err := db.RecordAuditEvent(ctx, id.Org, id.Sub, id.Email, action, target); err != nil {
		slog.Warn("audit_record_failed",
			"action", action, "org", id.Org, "error", err.Error(),
			"request_id", middleware.GetReqID(ctx))
	}
}

// tenantRequest wraps a message and attaches the caller's tenant header, or fails
// with CodeUnauthenticated when there is no identity.
func tenantRequest[T any](ctx context.Context, msg *T) (*connect.Request[T], error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity"))
	}
	req := connect.NewRequest(msg)
	req.Header().Set(headerTenant, id.Org)
	return req, nil
}

// connectorCallback is the public OAuth redirect target. The provider sends the
// user here with code + state; we hand them to the integrations service to
// exchange and store, then bounce the browser back to the app. It trusts the
// signed state, not a session (ADR-0027).
func (s *Server) connectorCallback(w http.ResponseWriter, r *http.Request) {
	connector := chi.URLParam(r, "connector")
	q := r.URL.Query()

	if providerErr := q.Get("error"); providerErr != "" {
		s.redirectToSources(w, r, "error="+providerErr)
		return
	}

	res, err := s.integrationInternal.CompleteConnection(r.Context(),
		connect.NewRequest(&integrationv1.CompleteConnectionRequest{
			Code:           q.Get("code"),
			State:          q.Get("state"),
			InstallationId: q.Get("installation_id"),
		}))
	if err != nil {
		slog.Warn("connector_callback_failed",
			"connector", connector, "error", err.Error(),
			"request_id", middleware.GetReqID(r.Context()))
		s.redirectToSources(w, r, "connect_error=1")
		return
	}
	// Hand back to the Sources list with a marker — the list shows the new source's
	// discovery progress and auto-opens its manage page once discovery finishes,
	// rather than parking the user on a page while the worker cold-starts.
	http.Redirect(w, r, s.webBaseURL+"/sources?connected="+res.Msg.GetConnection().GetId(), http.StatusFound)
}

func (s *Server) redirectToSources(w http.ResponseWriter, r *http.Request, query string) {
	http.Redirect(w, r, s.webBaseURL+"/sources?"+query, http.StatusFound)
}
