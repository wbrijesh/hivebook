package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"connectrpc.com/validate"

	"api/internal/auth"
	"api/internal/database"
	tenantv1 "api/internal/gen/hivebook/tenant/v1"
	"api/internal/gen/hivebook/tenant/v1/tenantv1connect"
)

// --- stubs ---------------------------------------------------------------

type fakeDB struct {
	tenant     database.Tenant
	getErr     error
	onboardErr error
}

func (f *fakeDB) Health() map[string]string { return map[string]string{"status": "up"} }
func (f *fakeDB) Stats() sql.DBStats        { return sql.DBStats{} }
func (f *fakeDB) Close() error              { return nil }

func (f *fakeDB) GetOrCreateTenant(context.Context, string) (database.Tenant, error) {
	return f.tenant, f.getErr
}

func (f *fakeDB) CompleteOnboarding(context.Context, string, string, string, string, []string, string) (database.Tenant, error) {
	return f.tenant, f.onboardErr
}

func (f *fakeDB) UpdateTenantProfile(context.Context, string, string, string, []string, string) (database.Tenant, error) {
	return f.tenant, f.onboardErr
}

func (f *fakeDB) RecordAuditEvent(context.Context, string, string, string, string, string) error {
	return nil
}

func (f *fakeDB) ListAuditEvents(context.Context, string, int32, int32) ([]database.AuditEvent, int32, error) {
	return nil, 0, nil
}

func (f *fakeDB) GetFeatureFlags(context.Context, string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (f *fakeDB) SetFeatureFlag(context.Context, string, string, bool) error {
	return nil
}

func (f *fakeDB) RemoveFeatureFlag(context.Context, string, string) error {
	return nil
}

// fakeAuth stands in for the Authenticator: the configured identity/error is
// returned regardless of the header, so tests drive the interceptor's mapping.
type fakeAuth struct {
	id  auth.Identity
	err error
}

func (f *fakeAuth) Authenticate(context.Context, string) (auth.Identity, error) {
	return f.id, f.err
}

// newTestClient stands up the real Connect handler — auth + protovalidate
// interceptors and all — on an httptest server and returns a generated client,
// so tests exercise the same wiring that serves production.
func newTestClient(t *testing.T, db database.Service, authn authenticator) tenantv1connect.TenantServiceClient {
	t.Helper()
	validator := validate.NewInterceptor()
	path, h := tenantv1connect.NewTenantServiceHandler(
		&tenantService{db: db},
		connect.WithInterceptors(newAuthInterceptor(authn), validator),
	)
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return tenantv1connect.NewTenantServiceClient(srv.Client(), srv.URL)
}

// authedRequest wraps a message with a bearer header so it passes the auth
// interceptor (the fake ignores the value; it just needs to be present).
func authedRequest[T any](msg *T) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer test")
	return req
}

// --- GetSession ----------------------------------------------------------

func TestGetSession_OK(t *testing.T) {
	client := newTestClient(t,
		&fakeDB{tenant: database.Tenant{ID: "t1", UseCases: []string{}}},
		&fakeAuth{id: auth.Identity{Sub: "u1", Name: "U", Email: "u@x.com", Org: "o1"}},
	)
	res, err := client.GetSession(context.Background(), authedRequest(&tenantv1.GetSessionRequest{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Msg.GetUser().GetId() != "u1" || res.Msg.GetTenant().GetId() != "t1" {
		t.Fatalf("unexpected response: %+v", res.Msg)
	}
}

func TestGetSession_Unauthenticated(t *testing.T) {
	client := newTestClient(t, &fakeDB{}, &fakeAuth{err: auth.ErrUnauthenticated})
	_, err := client.GetSession(context.Background(), authedRequest(&tenantv1.GetSessionRequest{}))
	if got := connect.CodeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("got code %v, want Unauthenticated", got)
	}
}

func TestGetSession_ProviderNotReady(t *testing.T) {
	client := newTestClient(t, &fakeDB{}, &fakeAuth{err: auth.ErrProviderNotReady})
	_, err := client.GetSession(context.Background(), authedRequest(&tenantv1.GetSessionRequest{}))
	if got := connect.CodeOf(err); got != connect.CodeUnavailable {
		t.Fatalf("got code %v, want Unavailable", got)
	}
}

func TestGetSession_DBError(t *testing.T) {
	client := newTestClient(t,
		&fakeDB{getErr: errors.New("boom")},
		&fakeAuth{id: auth.Identity{Org: "o1"}},
	)
	_, err := client.GetSession(context.Background(), authedRequest(&tenantv1.GetSessionRequest{}))
	if got := connect.CodeOf(err); got != connect.CodeInternal {
		t.Fatalf("got code %v, want Internal", got)
	}
}

// --- CompleteOnboarding (table-driven) -----------------------------------

func TestCompleteOnboarding(t *testing.T) {
	okTenant := database.Tenant{ID: "t1", UseCases: []string{}}
	cases := []struct {
		name string
		req  *tenantv1.CompleteOnboardingRequest
		auth *fakeAuth
		db   *fakeDB
		want connect.Code // 0 means success
	}{
		{
			"ok",
			&tenantv1.CompleteOnboardingRequest{Region: "us-east", Name: "A", Size: "1-10", UseCases: []string{"support"}},
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{tenant: okTenant}, 0,
		},
		{
			"missing region → InvalidArgument (protovalidate)",
			&tenantv1.CompleteOnboardingRequest{Name: "A"},
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{tenant: okTenant}, connect.CodeInvalidArgument,
		},
		{
			"region immutable → FailedPrecondition",
			&tenantv1.CompleteOnboardingRequest{Region: "eu-central"},
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{onboardErr: database.ErrRegionImmutable}, connect.CodeFailedPrecondition,
		},
		{
			"unauthenticated",
			&tenantv1.CompleteOnboardingRequest{Region: "us-east"},
			&fakeAuth{err: auth.ErrUnauthenticated}, &fakeDB{}, connect.CodeUnauthenticated,
		},
		{
			"db error → Internal",
			&tenantv1.CompleteOnboardingRequest{Region: "us-east"},
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{onboardErr: errors.New("boom")}, connect.CodeInternal,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t, tc.db, tc.auth)
			_, err := client.CompleteOnboarding(context.Background(), authedRequest(tc.req))
			if tc.want == 0 { // 0 is not a Connect code; use it as the success sentinel
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if got := connect.CodeOf(err); got != tc.want {
				t.Fatalf("got code %v, want %v (err=%v)", got, tc.want, err)
			}
		})
	}
}
