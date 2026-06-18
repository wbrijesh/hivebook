// Package server is the integrations service's Connect surface: the user-facing
// IntegrationService (proxied to by the API) and the internal
// IntegrationInternalService (the OAuth code exchange). Internal-only — the tenant
// arrives as a header, trusted from the API; CompleteConnection trusts the signed
// state instead (design-doc 0009).
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	enums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/types/known/timestamppb"

	"integrations/internal/connectors"
	"integrations/internal/database"
	"integrations/internal/database/gen"
	"integrations/internal/events"
	integrationv1 "integrations/internal/gen/hivebook/integration/v1"
	"integrations/internal/gen/hivebook/integration/v1/integrationv1connect"
	"integrations/internal/oauth"
	"integrations/internal/storage"
	"integrations/internal/temporal"
	"integrations/internal/workflow"
)

// Artifact-list paging defaults (constants-first, ADR-0011).
const (
	defaultArtifactLimit = 50
	maxArtifactLimit     = 200
)

const stateTTL = 10 * time.Minute

// Auto-sync interval bounds, enforced in the UpdateSyncSchedule handler (not
// buf.validate) so they can move without a proto edit (constants-first, ADR-0011).
const (
	// minSyncIntervalSeconds is the floor on the auto-sync cadence.
	minSyncIntervalSeconds = 60 // TEMP: dev-only 1-minute floor for testing; raise to 900 (15 min) before GA
	// maxSyncIntervalSeconds caps the cadence at 30 days.
	maxSyncIntervalSeconds = 2592000 // 30 days
)

// githubAdminClient bounds the GitHub App admin calls (install/uninstall/resolve,
// installation-account lookup) so a slow/hanging GitHub never wedges the RPC —
// http.DefaultClient has no timeout (design-doc 0011 P3-7).
var githubAdminClient = &http.Client{Timeout: 20 * time.Second}

// Server implements both Connect services over the data layer.
type Server struct {
	store        *database.Store
	reg          *connectors.Registry
	cipher       *oauth.Cipher
	signer       *oauth.Signer
	creds        map[string]connectors.Creds
	broker       *events.Broker
	objects      *storage.Store // serve artifact content through our API
	temporal     client.Client  // the integrations service is a Temporal CLIENT (design-doc 0012)
	callbackBase string         // e.g. https://api.hivebook.localhost — the API hosts the callback
}

// New wires the server. The Temporal client makes this service the control-plane
// CLIENT of the per-connection ConnectionWorkflow: RPCs translate into signals
// (design-doc 0012, ADR-0034).
func New(store *database.Store, reg *connectors.Registry, cipher *oauth.Cipher, signer *oauth.Signer, creds map[string]connectors.Creds, broker *events.Broker, objects *storage.Store, tc client.Client, callbackBase string) *Server {
	return &Server{
		store: store, reg: reg, cipher: cipher, signer: signer, creds: creds,
		broker: broker, objects: objects, temporal: tc, callbackBase: callbackBase,
	}
}

// connWorkflowID is the fixed, stable workflow id per connection: exactly one durable
// ConnectionWorkflow maps to one connection (design-doc 0012). SignalWithStart uses it
// to keep a single running instance — a start over a running one just delivers the signal.
func connWorkflowID(connectionID string) string { return "connection-" + connectionID }

// connWorkflowInput seeds the ConnectionWorkflow (tiny, payload-free — design-doc 0012).
func connWorkflowInput(c gen.ConnectorConnection) workflow.ConnectionWorkflowInput {
	return workflow.ConnectionWorkflowInput{
		ConnectionID: c.ID,
		TenantID:     c.TenantID,
		ConnectorID:  c.ConnectorID,
	}
}

// startConnectionOptions keeps ONE running ConnectionWorkflow per connection: a
// SignalWithStart over an already-running instance just delivers the signal (no second
// workflow), and a terminated/completed id may be re-used by the next connect.
func startConnectionOptions(connectionID string) client.StartWorkflowOptions {
	return client.StartWorkflowOptions{
		ID:                       connWorkflowID(connectionID),
		TaskQueue:                temporal.ConnectionQueue,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}
}

// signalConnection signal-with-starts the connection's workflow: it starts the durable
// lifecycle if idle and delivers signalName (idempotent "start + do X"). A nil
// signalArg is fine for the payload-free signals.
func (s *Server) signalConnection(ctx context.Context, c gen.ConnectorConnection, signalName string, signalArg any) error {
	if s.temporal == nil { // no orchestrator wired (unit tests of the authz/data paths)
		return nil
	}
	_, err := s.temporal.SignalWithStartWorkflow(ctx, connWorkflowID(c.ID), signalName, signalArg,
		startConnectionOptions(c.ID), workflow.ConnectionWorkflow, connWorkflowInput(c))
	return err
}

// provisionConnection brings a freshly-connected connection's workflow to life: it
// signal-with-starts the ConnectionWorkflow IDLE (it waits for triggers — no auto-sync)
// and delivers a discover-only RefreshCatalog so the project catalog populates for
// selection before the first sync (design-doc 0012). Best-effort and fire-and-forget:
// a Temporal blip must not fail the connect — the catalog also refreshes on ListProjects.
func (s *Server) provisionConnection(ctx context.Context, c gen.ConnectorConnection) {
	if err := s.signalConnection(ctx, c, workflow.SignalRefreshCatalog, nil); err != nil {
		slog.Warn("provision_connection_failed", "error", err.Error(), "connection_id", c.ID)
	}
}

// Handler builds the HTTP handler: both Connect services, /health, /metrics, h2c.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(accessLog)

	interceptors := connect.WithInterceptors(newTenantInterceptor(), validate.NewInterceptor())
	pubPath, pubHandler := integrationv1connect.NewIntegrationServiceHandler(s, interceptors)
	r.Mount(pubPath, pubHandler)
	intPath, intHandler := integrationv1connect.NewIntegrationInternalServiceHandler(s, interceptors)
	r.Mount(intPath, intHandler)

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"hivebook-integrations"}`))
	})
	r.Get("/health", s.health)
	r.Handle("/metrics", promhttp.Handler())

	return h2c.NewHandler(r, &http2.Server{IdleTimeout: time.Minute, MaxConcurrentStreams: 250})
}

// accessLog emits one structured JSON line per request (standards/go/logging),
// shipped to VictoriaLogs like the rest of the platform.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	h := s.store.Health()
	code := http.StatusOK
	if h["status"] != "up" {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(h)
}

// --- IntegrationService (user-facing) ------------------------------------

func (s *Server) ListConnectors(_ context.Context, _ *connect.Request[integrationv1.ListConnectorsRequest]) (*connect.Response[integrationv1.ListConnectorsResponse], error) {
	out := make([]*integrationv1.Connector, 0, len(s.reg.All()))
	for _, c := range s.reg.All() {
		out = append(out, &integrationv1.Connector{Id: c.ID(), Name: c.Name(), Archetype: c.Archetype()})
	}
	return connect.NewResponse(&integrationv1.ListConnectorsResponse{Connectors: out}), nil
}

func (s *Server) ListConnections(ctx context.Context, _ *connect.Request[integrationv1.ListConnectionsRequest]) (*connect.Response[integrationv1.ListConnectionsResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.Q.ListConnections(ctx, tenant)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("list connections failed"))
	}
	out := make([]*integrationv1.Connection, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProtoConnection(row))
	}
	return connect.NewResponse(&integrationv1.ListConnectionsResponse{Connections: out}), nil
}

func (s *Server) GetAuthorizeUrl(ctx context.Context, req *connect.Request[integrationv1.GetAuthorizeUrlRequest]) (*connect.Response[integrationv1.GetAuthorizeUrlResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	region, ok := regionFrom(ctx)
	if !ok {
		// A connection's residency region must be set before it can be created —
		// it anchors the object-storage bucket (ADR-0024, review #12).
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tenant region not set"))
	}
	conn, ok := s.reg.Get(req.Msg.GetConnectorId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown connector"))
	}
	state, err := s.signer.Sign(tenant, region, conn.ID(), stateTTL)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not start authorization"))
	}
	// The connector decides where to send the user — OAuth consent, or (GitHub App)
	// the install page so repo access is granted in the same flow.
	url := conn.AuthorizeURL(s.credsFor(conn.ID()), state, s.redirectURL(conn.ID()))
	return connect.NewResponse(&integrationv1.GetAuthorizeUrlResponse{Url: url}), nil
}

// TriggerSync is "Sync now": signal-with-start the connection's ConnectionWorkflow so
// it starts (if idle) and runs a sync immediately (design-doc 0012). Idempotent — a
// trigger mid-run coalesces into one rerun; the workflow handles concurrency, so the
// old 'stopping' guard is gone.
func (s *Server) TriggerSync(ctx context.Context, req *connect.Request[integrationv1.TriggerSyncRequest]) (*connect.Response[integrationv1.TriggerSyncResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	// Prove ownership before signalling — a connection id is not a secret (design-doc 0011 P0).
	conn, err := s.store.Q.GetConnection(ctx, gen.GetConnectionParams{ID: req.Msg.GetConnectionId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}
	if err := s.signalConnection(ctx, conn, workflow.SignalTriggerSync, nil); err != nil {
		slog.Error("trigger_sync_signal_failed", "error", err.Error(), "connection_id", conn.ID)
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not start sync"))
	}
	_ = s.store.NotifyConnectionChanged(ctx, tenant) // nudge watchers; the run flips status shortly
	return connect.NewResponse(&integrationv1.TriggerSyncResponse{}), nil
}

// StopSync cooperatively cancels an in-flight sync and settles it back to a resting
// state (ADR-0040). Already-synced work is kept; a later sync resumes from each unit's
// cursor. A signal to a workflow that isn't running is a no-op (NotFound) — treat that
// as success: there is nothing to stop.
func (s *Server) StopSync(ctx context.Context, req *connect.Request[integrationv1.StopSyncRequest]) (*connect.Response[integrationv1.StopSyncResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := s.store.Q.GetConnection(ctx, gen.GetConnectionParams{ID: req.Msg.GetConnectionId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}
	if s.temporal != nil {
		if err := s.temporal.SignalWorkflow(ctx, connWorkflowID(conn.ID), "", workflow.SignalStopSync, nil); err != nil {
			var notFound *serviceerror.NotFound
			if errors.As(err, &notFound) {
				return connect.NewResponse(&integrationv1.StopSyncResponse{}), nil // nothing running
			}
			slog.Error("stop_sync_signal_failed", "error", err.Error(), "connection_id", conn.ID)
			return nil, connect.NewError(connect.CodeInternal, errors.New("could not stop sync"))
		}
	}
	return connect.NewResponse(&integrationv1.StopSyncResponse{}), nil
}

func (s *Server) Disconnect(ctx context.Context, req *connect.Request[integrationv1.DisconnectRequest]) (*connect.Response[integrationv1.DisconnectResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	// Prove ownership before any destructive action — a connection id is not a
	// secret, so we must never act on one the caller doesn't own (design-doc 0011
	// P0). GetConnection is tenant-scoped; a miss is NotFound, not a silent no-op.
	conn, err := s.store.Q.GetConnection(ctx, gen.GetConnectionParams{ID: req.Msg.GetConnectionId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}

	// Soft delete: hide the connection and mark its files for purge at midnight IST.
	// Both are tenant-scoped; the row count guards against acting on a non-owned id.
	rows, err := s.store.Q.DeleteConnection(ctx, gen.DeleteConnectionParams{ID: conn.ID, TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not disconnect"))
	}
	if rows == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}
	// Immediate erasure (ADR-0039): the user revoked the connection, so mark ALL its
	// artifacts purge_pending with no grace (deadline = now). This is the same tenant-
	// scoped query the MarkConnectionPurge activity wraps; the MaintenanceWorkflow's GC
	// then erases the bytes. Hide them from Files now too (SoftDelete).
	_ = s.store.Q.SoftDeleteConnectionArtifacts(ctx, gen.SoftDeleteConnectionArtifactsParams{ConnectionID: conn.ID, TenantID: tenant})
	_ = s.store.Q.MarkConnectionArtifactsPurgePending(ctx, gen.MarkConnectionArtifactsPurgePendingParams{ConnectionID: conn.ID, TenantID: tenant})

	// Tell the workflow to stop + end (it soft-deletes nothing — we already did). A
	// missing workflow is a no-op: the connection was never synced. Best-effort.
	if s.temporal != nil {
		if err := s.temporal.SignalWorkflow(ctx, connWorkflowID(conn.ID), "", workflow.SignalDisconnect, nil); err != nil {
			var notFound *serviceerror.NotFound
			if !errors.As(err, &notFound) {
				slog.Warn("disconnect_signal_failed", "error", err.Error(), "connection_id", conn.ID)
			}
		}
	}
	_ = s.store.NotifyConnectionChanged(ctx, tenant) // drop the row from watchers live

	// Actually uninstall the GitHub App (not just drop our side) — otherwise the
	// install lingers and reconnecting reads "Update access" forever, forcing a
	// manual cleanup in GitHub settings. Best-effort. We remove both the id we
	// stored AND whatever installation is currently live on the account, since the
	// stored id can drift if the user reinstalled/updated access outside a fresh
	// connect (that mints a new id we never recorded).
	if cr := s.credsFor(conn.ConnectorID); cr.PrivateKey != "" && (conn.InstallationID != "" || conn.Account != "") {
		if key, err := connectors.ParseGitHubKey([]byte(cr.PrivateKey)); err == nil {
			done := map[string]bool{}
			uninstall := func(id string) {
				if id == "" || done[id] {
					return
				}
				done[id] = true
				if err := connectors.GitHubUninstall(ctx, githubAdminClient, cr.ClientID, key, id); err != nil {
					slog.Warn("github_uninstall_failed", "error", err.Error(), "installation_id", id, "account", conn.Account)
				} else {
					slog.Info("github_uninstalled", "installation_id", id, "account", conn.Account)
				}
			}
			uninstall(conn.InstallationID)
			if conn.Account != "" {
				if liveID, err := connectors.GitHubResolveInstallation(ctx, githubAdminClient, cr.ClientID, key, conn.Account); err != nil {
					slog.Warn("github_resolve_installation_failed", "error", err.Error(), "account", conn.Account)
				} else {
					uninstall(liveID)
				}
			}
		}
	}
	return connect.NewResponse(&integrationv1.DisconnectResponse{}), nil
}

func (s *Server) ListArtifacts(ctx context.Context, req *connect.Request[integrationv1.ListArtifactsRequest]) (*connect.Response[integrationv1.ListArtifactsResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	limit := int32(req.Msg.GetLimit())
	if limit <= 0 {
		limit = defaultArtifactLimit
	}
	if limit > maxArtifactLimit {
		limit = maxArtifactLimit
	}
	connID := req.Msg.GetConnectionId() // "" → all of the tenant's files
	rows, err := s.store.Q.ListArtifacts(ctx, gen.ListArtifactsParams{
		TenantID: tenant, RowLimit: limit, RowOffset: int32(req.Msg.GetOffset()),
		ConnectionID: connID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("list artifacts failed"))
	}
	total, err := s.store.Q.CountArtifacts(ctx, gen.CountArtifactsParams{
		TenantID: tenant, ConnectionID: connID,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("count artifacts failed"))
	}
	out := make([]*integrationv1.Artifact, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProtoArtifact(r))
	}
	return connect.NewResponse(&integrationv1.ListArtifactsResponse{Artifacts: out, Total: uint32(total)}), nil
}

// GetArtifactContent serves one artifact's bytes from object storage, scoped to
// the tenant. The frontend's View action goes through here — the object-storage
// URL is never exposed.
func (s *Server) GetArtifactContent(ctx context.Context, req *connect.Request[integrationv1.GetArtifactContentRequest]) (*connect.Response[integrationv1.GetArtifactContentResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	row, err := s.store.Q.GetArtifactForContent(ctx, gen.GetArtifactForContentParams{ID: req.Msg.GetId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("artifact not found"))
	}
	data, contentType, err := s.objects.Get(ctx, row.ObjectKey)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not read artifact"))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return connect.NewResponse(&integrationv1.GetArtifactContentResponse{
		Data: data, ContentType: contentType, Filename: row.ExternalID,
	}), nil
}

// WatchConnections streams a fresh snapshot of the tenant's connections on
// subscribe, then again on every change — driven by Postgres LISTEN/NOTIFY via
// the broker, so it idles until something actually changes (no polling).
func (s *Server) WatchConnections(ctx context.Context, _ *connect.Request[integrationv1.WatchConnectionsRequest], stream *connect.ServerStream[integrationv1.WatchConnectionsResponse]) error {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return err
	}
	signals, unsubscribe := s.broker.Subscribe(tenant)
	defer unsubscribe()

	if err := s.sendConnections(ctx, tenant, stream); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil // client went away
		case <-signals:
			if err := s.sendConnections(ctx, tenant, stream); err != nil {
				return err
			}
		}
	}
}

func (s *Server) sendConnections(ctx context.Context, tenant string, stream *connect.ServerStream[integrationv1.WatchConnectionsResponse]) error {
	rows, err := s.store.Q.ListConnections(ctx, tenant)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("list connections failed"))
	}
	out := make([]*integrationv1.Connection, 0, len(rows))
	for _, row := range rows {
		out = append(out, toProtoConnection(row))
	}
	return stream.Send(&integrationv1.WatchConnectionsResponse{Connections: out})
}

// ListProjects returns the connection's sync-unit catalog for selection. It fires a
// best-effort RefreshCatalog signal so the workflow re-discovers live (the worker then
// NOTIFYs and WatchProjects pushes the fresh list), then reads the current sync_unit
// rows so the call returns immediately without waiting on discovery (design-doc 0012).
func (s *Server) ListProjects(ctx context.Context, req *connect.Request[integrationv1.ListProjectsRequest]) (*connect.Response[integrationv1.ListProjectsResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := s.store.Q.GetConnection(ctx, gen.GetConnectionParams{ID: req.Msg.GetConnectionId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}
	// Best-effort live refresh: the result below is the current catalog; the refreshed
	// one arrives via WatchProjects when discovery settles.
	if err := s.signalConnection(ctx, conn, workflow.SignalRefreshCatalog, nil); err != nil {
		slog.Warn("list_projects_refresh_failed", "error", err.Error(), "connection_id", conn.ID)
	}
	// Optional server-side name filter — the manage page sends a debounced query; an
	// empty query means no filter. WatchProjects (the live stream) never filters.
	rows, err := s.store.Q.ListSyncUnits(ctx, gen.ListSyncUnitsParams{
		ConnectionID: conn.ID, TenantID: tenant, Query: strings.TrimSpace(req.Msg.GetQuery()),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("list projects failed"))
	}
	out := make([]*integrationv1.Project, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProtoProject(r))
	}
	return connect.NewResponse(&integrationv1.ListProjectsResponse{Projects: out}), nil
}

func (s *Server) SetProjectSelection(ctx context.Context, req *connect.Request[integrationv1.SetProjectSelectionRequest]) (*connect.Response[integrationv1.SetProjectSelectionResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	// Authorize the connection against the tenant before mutating its units.
	conn, err := s.store.Q.GetConnection(ctx, gen.GetConnectionParams{ID: req.Msg.GetConnectionId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}
	if err := s.store.Q.SetUnitSelection(ctx, gen.SetUnitSelectionParams{
		ConnectionID: conn.ID, SelectedIds: req.Msg.GetSelectedIds(),
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save selection"))
	}
	// Saving a selection configures the source — onboarding is complete. The frontend
	// routes a not-onboarded connection to its manage page, an onboarded one to details.
	if err := s.store.Q.MarkConnectionOnboarded(ctx, conn.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not mark onboarded"))
	}
	// Deselecting a unit marks its files for purge; reselecting resurrects them.
	_ = s.store.Q.SoftDeleteDeselectedUnitArtifacts(ctx, conn.ID)
	_ = s.store.Q.UndeleteSelectedUnitArtifacts(ctx, conn.ID)
	// Tell a running sync to honor the new selection at the next batch boundary.
	if err := s.signalConnection(ctx, conn, workflow.SignalUpdateSelection, nil); err != nil {
		slog.Warn("set_selection_signal_failed", "error", err.Error(), "connection_id", conn.ID)
	}
	_ = s.store.NotifyConnectionChanged(ctx, tenant) // refresh project watchers
	return connect.NewResponse(&integrationv1.SetProjectSelectionResponse{}), nil
}

// UpdateSyncSchedule sets the connection's auto-sync cadence (on/off + interval). It
// mirrors SetProjectSelection: authorize the connection against the tenant, write the
// row, then best-effort signal the workflow so it re-reads the schedule live (design-doc
// 0012). The interval bounds are enforced here (not buf.validate) so the temporary dev
// floor stays a Go constant; the floor only matters when auto-sync is enabled.
func (s *Server) UpdateSyncSchedule(ctx context.Context, req *connect.Request[integrationv1.UpdateSyncScheduleRequest]) (*connect.Response[integrationv1.UpdateSyncScheduleResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	// Authorize the connection against the tenant before mutating it (a connection id is
	// not a secret — design-doc 0011 P0).
	conn, err := s.store.Q.GetConnection(ctx, gen.GetConnectionParams{ID: req.Msg.GetConnectionId(), TenantID: tenant})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}
	interval := req.Msg.GetIntervalSeconds()
	if req.Msg.GetEnabled() && (interval < minSyncIntervalSeconds || interval > maxSyncIntervalSeconds) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sync interval out of range"))
	}
	if err := s.store.Q.UpdateSyncSchedule(ctx, gen.UpdateSyncScheduleParams{
		ID: conn.ID, AutoSyncEnabled: req.Msg.GetEnabled(), SyncIntervalSeconds: interval,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save sync schedule"))
	}
	// Tell the workflow to re-read the cadence and re-arm its timer at the next idle.
	if err := s.signalConnection(ctx, conn, workflow.SignalUpdateSchedule, nil); err != nil {
		slog.Warn("update_schedule_signal_failed", "error", err.Error(), "connection_id", conn.ID)
	}
	_ = s.store.NotifyConnectionChanged(ctx, tenant) // refresh connection watchers
	return connect.NewResponse(&integrationv1.UpdateSyncScheduleResponse{}), nil
}

// WatchProjects streams per-project status for one connection — same broker as
// WatchConnections (a project change NOTIFYs the tenant; this re-reads the list).
func (s *Server) WatchProjects(ctx context.Context, req *connect.Request[integrationv1.WatchProjectsRequest], stream *connect.ServerStream[integrationv1.WatchProjectsResponse]) error {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return err
	}
	connID := req.Msg.GetConnectionId()
	signals, unsubscribe := s.broker.Subscribe(tenant)
	defer unsubscribe()

	if err := s.sendProjects(ctx, tenant, connID, stream); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-signals:
			if err := s.sendProjects(ctx, tenant, connID, stream); err != nil {
				return err
			}
		}
	}
}

func (s *Server) sendProjects(ctx context.Context, tenant, connID string, stream *connect.ServerStream[integrationv1.WatchProjectsResponse]) error {
	rows, err := s.store.Q.ListSyncUnits(ctx, gen.ListSyncUnitsParams{ConnectionID: connID, TenantID: tenant})
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("list projects failed"))
	}
	out := make([]*integrationv1.Project, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProtoProject(r))
	}
	return stream.Send(&integrationv1.WatchProjectsResponse{Projects: out})
}

// --- IntegrationInternalService (API-only) -------------------------------

func (s *Server) CompleteConnection(ctx context.Context, req *connect.Request[integrationv1.CompleteConnectionRequest]) (*connect.Response[integrationv1.CompleteConnectionResponse], error) {
	st, err := s.signer.Verify(req.Msg.GetState())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid state"))
	}
	conn, ok := s.reg.Get(st.Connector)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown connector"))
	}

	// GitHub App installation flow: authenticate as the installation (no user
	// token). The redirect carries the installation id; we store it and mint
	// installation tokens from the App private key thereafter.
	cr := s.credsFor(conn.ID())
	if instID := req.Msg.GetInstallationId(); instID != "" && cr.PrivateKey != "" {
		return s.completeInstallation(ctx, st, conn, cr, instID)
	}

	cfg := conn.OAuthConfig(cr.ClientID, cr.ClientSecret, s.redirectURL(conn.ID()))
	tok, err := cfg.Exchange(ctx, req.Msg.GetCode())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code exchange failed"))
	}

	account, err := conn.Account(ctx, cfg.Client(ctx, tok))
	if err != nil {
		account = "" // non-fatal: a missing label shouldn't block the connection
	}

	access, err := s.cipher.EncryptString(tok.AccessToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not store token"))
	}
	refresh, err := s.cipher.EncryptString(tok.RefreshToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not store token"))
	}

	row, err := s.store.Q.UpsertConnection(ctx, gen.UpsertConnectionParams{
		TenantID:       st.Tenant,
		Region:         st.Region,
		ConnectorID:    conn.ID(),
		Account:        account,
		AccessToken:    access,
		RefreshToken:   refresh,
		TokenExpiry:    nullTime(tok.Expiry),
		Scopes:         strings.Join(conn.Scopes(), " "),
		InstallationID: "",
		ExternalID:     account, // OAuth connectors key on the account
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save connection"))
	}

	// Reconnecting before the purge resurrects any files pending deletion.
	_ = s.store.Q.UndeleteConnectionArtifacts(ctx, gen.UndeleteConnectionArtifactsParams{ConnectionID: row.ID, TenantID: row.TenantID})
	// Start the connection's ConnectionWorkflow IDLE and discover the catalog (no
	// auto-sync — the first sync runs only when the user clicks Sync now). The catalog
	// also refreshes on ListProjects, so this is best-effort.
	s.provisionConnection(ctx, row)
	return connect.NewResponse(&integrationv1.CompleteConnectionResponse{Connection: toProtoConnection(row)}), nil
}

// completeInstallation stores a GitHub App connection from the install redirect:
// no user token — the installation id drives installation-token auth. The account
// label comes from the installation itself.
func (s *Server) completeInstallation(ctx context.Context, st oauth.State, conn connectors.Connector, cr connectors.Creds, installationID string) (*connect.Response[integrationv1.CompleteConnectionResponse], error) {
	key, err := connectors.ParseGitHubKey([]byte(cr.PrivateKey))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("app key invalid"))
	}
	account, _, err := connectors.GitHubInstallationAccount(ctx, githubAdminClient, cr.ClientID, key, installationID)
	if err != nil {
		slog.Warn("github_installation_account_failed", "error", err.Error(), "installation_id", installationID)
		account = "" // non-fatal label
	}
	row, err := s.store.Q.UpsertConnection(ctx, gen.UpsertConnectionParams{
		TenantID:       st.Tenant,
		Region:         st.Region,
		ConnectorID:    conn.ID(),
		Account:        account,
		AccessToken:    nil,
		RefreshToken:   nil,
		TokenExpiry:    nullTime(time.Time{}),
		Scopes:         "",
		InstallationID: installationID,
		ExternalID:     installationID, // GitHub App keys on the installation
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save connection"))
	}
	_ = s.store.Q.UndeleteConnectionArtifacts(ctx, gen.UndeleteConnectionArtifactsParams{ConnectionID: row.ID, TenantID: row.TenantID})
	// Start the workflow idle + discover the catalog (see CompleteConnection) — Sync now
	// starts the first sync.
	s.provisionConnection(ctx, row)
	return connect.NewResponse(&integrationv1.CompleteConnectionResponse{Connection: toProtoConnection(row)}), nil
}

// CompleteTokenConnection creates a connection from a user-supplied token (e.g. a
// GitHub PAT) — no OAuth redirect. The token is validated against the source, then
// stored as the connection's access token (no refresh, no installation), so the
// existing token path uses it directly as a bearer. Tenant + region come from the
// internal-hop headers; the API enforces the feature flag before calling this.
func (s *Server) CompleteTokenConnection(ctx context.Context, req *connect.Request[integrationv1.CompleteTokenConnectionRequest]) (*connect.Response[integrationv1.CompleteTokenConnectionResponse], error) {
	tenant, err := requireTenant(ctx)
	if err != nil {
		return nil, err
	}
	region, _ := regionFrom(ctx)
	conn, ok := s.reg.Get(req.Msg.GetConnectorId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("unknown connector"))
	}
	token := req.Msg.GetToken()

	// Validate the token by resolving the account it belongs to.
	login, err := conn.Account(ctx, &http.Client{Transport: bearerRoundTripper{token: token}})
	if err != nil {
		slog.Warn("token_connection_rejected", "connector", conn.ID(), "error", err.Error())
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("the source rejected this token"))
	}
	// The user's chosen name is the label + identity (so several token sources can
	// coexist); fall back to the token owner's login if blank. A fine-grained PAT's
	// /user login is the token owner, which often isn't the repo owner — hence the
	// name field.
	label := req.Msg.GetName()
	if label == "" {
		label = login
	}

	access, err := s.cipher.EncryptString(token)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not store token"))
	}
	refresh, err := s.cipher.EncryptString("") // no refresh for a PAT; stored so decode() yields ""
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not store token"))
	}
	row, err := s.store.Q.UpsertConnection(ctx, gen.UpsertConnectionParams{
		TenantID:       tenant,
		Region:         region,
		ConnectorID:    conn.ID(),
		Account:        label,
		AccessToken:    access,
		RefreshToken:   refresh,
		TokenExpiry:    nullTime(time.Time{}),
		Scopes:         "",
		InstallationID: "",
		ExternalID:     label, // token connections key on the chosen name
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("could not save connection"))
	}
	_ = s.store.Q.UndeleteConnectionArtifacts(ctx, gen.UndeleteConnectionArtifactsParams{ConnectionID: row.ID, TenantID: row.TenantID})
	// Start the workflow idle + discover the catalog (see CompleteConnection).
	s.provisionConnection(ctx, row)
	return connect.NewResponse(&integrationv1.CompleteTokenConnectionResponse{Connection: toProtoConnection(row)}), nil
}

// ConnectWithToken (user-facing) delegates to CompleteTokenConnection — tenant +
// region come from the request context. The API path calls CompleteTokenConnection
// directly after enforcing the feature flag.
func (s *Server) ConnectWithToken(ctx context.Context, req *connect.Request[integrationv1.ConnectWithTokenRequest]) (*connect.Response[integrationv1.ConnectWithTokenResponse], error) {
	res, err := s.CompleteTokenConnection(ctx, connect.NewRequest(&integrationv1.CompleteTokenConnectionRequest{
		ConnectorId: req.Msg.GetConnectorId(), Token: req.Msg.GetToken(),
	}))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&integrationv1.ConnectWithTokenResponse{Connection: res.Msg.GetConnection()}), nil
}

// --- helpers -------------------------------------------------------------

func (s *Server) credsFor(id string) connectors.Creds { return s.creds[id] }

// bearerRoundTripper adds a bearer Authorization header — used to validate a
// user-supplied token against the source before storing it.
type bearerRoundTripper struct{ token string }

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func (s *Server) redirectURL(connectorID string) string {
	return strings.TrimRight(s.callbackBase, "/") + "/connectors/" + connectorID + "/callback"
}

func toProtoConnection(c gen.ConnectorConnection) *integrationv1.Connection {
	pc := &integrationv1.Connection{
		Id:                  c.ID,
		ConnectorId:         c.ConnectorID,
		Status:              statusToProto(c.Status),
		Account:             c.Account,
		LastError:           c.LastError,
		Onboarded:           c.Onboarded,
		AutoSyncEnabled:     c.AutoSyncEnabled,
		SyncIntervalSeconds: c.SyncIntervalSeconds,
	}
	if c.LastSuccessAt.Valid {
		pc.LastSyncedAt = timestamppb.New(c.LastSuccessAt.Time)
	}
	if c.CatalogDiscoveredAt.Valid {
		pc.CatalogDiscoveredAt = timestamppb.New(c.CatalogDiscoveredAt.Time)
	}
	return pc
}

func toProtoArtifact(a gen.ListArtifactsRow) *integrationv1.Artifact {
	return &integrationv1.Artifact{
		Id:               a.ID,
		ConnectorId:      a.Source,
		SourceNativeKind: a.SourceNativeKind,
		ContainerName:    a.ContainerName,
		ExternalId:       a.ExternalID,
		SourceUrl:        a.SourceUrl,
		IngestedAt:       timestamppb.New(a.IngestedAt),
		ConnectionId:     a.ConnectionID,
	}
}

// toProtoProject maps a sync_unit row to the proto Project (design-doc 0012: a project
// IS a sync_unit). The proto Project has no kind field, so kind is dropped. last_synced
// is the unit's committed_at — when its artifacts first became visible (ADR-0033).
func toProtoProject(p gen.ListSyncUnitsRow) *integrationv1.Project {
	pr := &integrationv1.Project{
		Id:            p.ExternalID,
		Name:          p.Name,
		Selected:      p.Selected,
		Status:        p.Status,
		ArtifactCount: uint32(p.ArtifactCount),
		LastError:     p.LastError,
		TotalEstimate: uint32(p.TotalEstimate),
	}
	if p.CommittedAt.Valid {
		pr.LastSyncedAt = timestamppb.New(p.CommittedAt.Time)
	}
	if p.TotalEstimateAt.Valid {
		pr.TotalEstimateAt = timestamppb.New(p.TotalEstimateAt.Time)
	}
	return pr
}

func statusToProto(s string) integrationv1.ConnectionStatus {
	switch s {
	case "connected":
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_CONNECTED
	case "syncing":
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_SYNCING
	case "backfilling":
		// An initial backfill is still in flight (design-doc 0012) — to the UI that's
		// actively syncing.
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_SYNCING
	case "connected_warnings":
		// Settled and visible, just flagged (a poison unit backing off) — reads as
		// connected; lastError carries the detail.
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_CONNECTED
	case "stopping":
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_STOPPING
	case "error":
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_ERROR
	case "needs_reauth":
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_NEEDS_REAUTH
	default:
		return integrationv1.ConnectionStatus_CONNECTION_STATUS_UNSPECIFIED
	}
}

func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}
