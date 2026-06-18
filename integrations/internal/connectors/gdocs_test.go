package connectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoogleDocsDiscoverUnits(t *testing.T) {
	g := NewGoogleDocs()
	cs, err := g.DiscoverUnits(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].ID != "drive" {
		t.Fatalf("expected a single drive unit, got %+v", cs)
	}
}

func TestGoogleDocsSyncUnit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/drive/v3/files", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("q"), "application/vnd.google-apps.document") {
			t.Errorf("unexpected query: %s", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("includeItemsFromAllDrives") != "true" {
			t.Errorf("expected Shared Drives to be included")
		}
		_, _ = w.Write([]byte(`{"files":[
			{"id":"doc1","name":"Runbook","modifiedTime":"2026-06-01T10:00:00Z","createdTime":"2026-05-01T09:00:00Z","webViewLink":"https://docs.google.com/d/doc1","parents":["folderA"]}
		]}`))
	})
	mux.HandleFunc("/drive/v3/files/doc1/export", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<h1>Refunds</h1><p>policy</p>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := &GoogleDocs{driveBaseURL: srv.URL, userinfoBaseURL: srv.URL}
	var got []Artifact
	done, err := g.SyncUnit(context.Background(), srv.Client(), Unit{ID: "drive"}, "", func(_ context.Context, a Artifact) error {
		got = append(got, a)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("the stream drained, should report done")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(got))
	}
	a := got[0]
	if a.ExternalID != "doc1" || a.SourceNativeKind != "gdoc" {
		t.Fatalf("unexpected artifact: %+v", a)
	}
	if a.Container.ID != "folderA" || a.Container.Kind != "drive_folder" {
		t.Fatalf("unexpected manifest container: %+v", a.Container)
	}
	if string(a.Raw) != "<h1>Refunds</h1><p>policy</p>" {
		t.Fatalf("unexpected raw: %q", a.Raw)
	}
	// The item's cursor is its modifiedTime — the value the emit layer advances to.
	if a.Cursor != "2026-06-01T10:00:00Z" {
		t.Fatalf("unexpected item cursor: %q", a.Cursor)
	}
}

func TestGoogleDocsAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v2/userinfo" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"email":"ops@acme.com"}`))
	}))
	defer srv.Close()

	g := &GoogleDocs{driveBaseURL: srv.URL, userinfoBaseURL: srv.URL}
	acct, err := g.Account(context.Background(), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if acct != "ops@acme.com" {
		t.Fatalf("unexpected account: %q", acct)
	}
}

func TestHTTPGetClassifiesErrors(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		check   func(error) bool
	}{
		{"rate limit 429", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
		}, func(err error) bool { e, ok := AsRateLimit(err); return ok && e.RetryAfter == 5e9 }},
		{"auth 401", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}, func(err error) bool { _, ok := AsAuth(err); return ok }},
		{"forbidden rate limit", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		}, func(err error) bool { _, ok := AsRateLimit(err); return ok }},
		{"forbidden secondary limit body", func(w http.ResponseWriter, _ *http.Request) {
			// A GitHub secondary limit: 403 with no standard headers, only a body.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit"}`))
		}, func(err error) bool { _, ok := AsRateLimit(err); return ok }},
		{"forbidden restricted resource", func(w http.ResponseWriter, _ *http.Request) {
			// A plain 403 (restricted repo / SAML): per-resource StatusError, NOT a
			// connection-terminal AuthError (H3 fix — only 401 is needs_reauth).
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Resource protected by organization SAML enforcement"}`))
		}, func(err error) bool {
			se, ok := AsStatus(err)
			if _, isAuth := AsAuth(err); isAuth {
				return false
			}
			return ok && se.Status == http.StatusForbidden
		}},
		{"oversize", func(w http.ResponseWriter, _ *http.Request) {
			buf := make([]byte, maxArtifactBytes+10)
			_, _ = w.Write(buf)
		}, func(err error) bool { return err != nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			_, err := httpGet(context.Background(), srv.Client(), srv.URL)
			if !tc.check(err) {
				t.Fatalf("error not classified as expected: %v", err)
			}
		})
	}
}
