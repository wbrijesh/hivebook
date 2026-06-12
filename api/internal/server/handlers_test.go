package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api/internal/auth"
	"api/internal/database"
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

type fakeAuth struct {
	id  auth.Identity
	err error
}

func (f *fakeAuth) Middleware(next http.Handler) http.Handler { return next }
func (f *fakeAuth) Identity(context.Context, *http.Request) (auth.Identity, error) {
	return f.id, f.err
}

// --- tests ---------------------------------------------------------------

func TestMeHandler_OK(t *testing.T) {
	s := &Server{
		db:   &fakeDB{tenant: database.Tenant{ID: "t1", UseCases: []string{}}},
		auth: &fakeAuth{id: auth.Identity{Sub: "u1", Name: "U", Email: "u@x.com", Org: "o1"}},
	}
	rec := httptest.NewRecorder()
	s.meHandler(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	var resp meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.User.ID != "u1" || resp.Tenant.ID != "t1" {
		t.Fatalf("unexpected body: %s", rec.Body)
	}
}

func TestMeHandler_IdentityError(t *testing.T) {
	s := &Server{db: &fakeDB{}, auth: &fakeAuth{err: errors.New("no identity")}}
	rec := httptest.NewRecorder()
	s.meHandler(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestTenantHandler(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		s := &Server{
			db:   &fakeDB{tenant: database.Tenant{ID: "t1", UseCases: []string{}}},
			auth: &fakeAuth{id: auth.Identity{Org: "o1"}},
		}
		rec := httptest.NewRecorder()
		s.tenantHandler(rec, httptest.NewRequest(http.MethodGet, "/api/tenant", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	})
	t.Run("db error → 500", func(t *testing.T) {
		s := &Server{
			db:   &fakeDB{getErr: errors.New("boom")},
			auth: &fakeAuth{id: auth.Identity{Org: "o1"}},
		}
		rec := httptest.NewRecorder()
		s.tenantHandler(rec, httptest.NewRequest(http.MethodGet, "/api/tenant", nil))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestOnboardingHandler(t *testing.T) {
	okTenant := database.Tenant{ID: "t1", UseCases: []string{}}
	cases := []struct {
		name string
		body string
		auth *fakeAuth
		db   *fakeDB
		want int
	}{
		{"ok", `{"region":"us-east","name":"A","size":"1-10","useCases":["support"]}`,
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{tenant: okTenant}, http.StatusOK},
		{"missing region → 400", `{"name":"A"}`,
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{}, http.StatusBadRequest},
		{"bad json → 400", `{`,
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{}, http.StatusBadRequest},
		{"region immutable → 409", `{"region":"eu-central"}`,
			&fakeAuth{id: auth.Identity{Org: "o1"}}, &fakeDB{onboardErr: database.ErrRegionImmutable}, http.StatusConflict},
		{"identity error → 422", `{"region":"us-east"}`,
			&fakeAuth{err: errors.New("no identity")}, &fakeDB{}, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{db: tc.db, auth: tc.auth}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/tenant/onboarding", strings.NewReader(tc.body))
			s.onboardingHandler(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d (body=%s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}
