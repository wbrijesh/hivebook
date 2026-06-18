package connectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// With an installation token, discovery lists ALL granted repos
// (/installation/repositories — not user-filtered) AND the account's Projects v2
// boards (GraphQL).
func TestGitHubDiscoverUnits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/installation/repositories":
			_, _ = w.Write([]byte(`{"total_count":2,"repositories":[
				{"full_name":"acme/api","owner":{"login":"acme","type":"Organization"}},
				{"full_name":"acme/web","owner":{"login":"acme","type":"Organization"}}]}`))
		case "/graphql":
			_, _ = w.Write([]byte(`{"data":{"organization":{"projectsV2":{"nodes":[{"id":"PVT_1","title":"Roadmap","number":7}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	cs, err := g.DiscoverUnits(context.Background(), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	var repos, projects int
	for _, c := range cs {
		switch c.Kind {
		case "github_repo":
			repos++
		case "github_project":
			projects++
			if c.ID != "PVT_1" || c.Name != "Roadmap" {
				t.Fatalf("unexpected project container: %+v", c)
			}
		}
	}
	if repos != 2 || projects != 1 {
		t.Fatalf("expected 2 repos + 1 project, got %d repos / %d projects: %+v", repos, projects, cs)
	}
}

// Project discovery is best-effort: a GraphQL permission error skips projects but
// repos still come through.
func TestGitHubDiscoverProjectsBestEffort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/installation/repositories":
			_, _ = w.Write([]byte(`{"repositories":[{"full_name":"acme/api","owner":{"login":"acme","type":"Organization"}}]}`))
		case "/graphql":
			_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"Resource not accessible by integration"}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	cs, err := g.DiscoverUnits(context.Background(), srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Kind != "github_repo" {
		t.Fatalf("expected just the repo when projects are forbidden, got %+v", cs)
	}
}

// A Project v2 board syncs its items via GraphQL.
func TestGitHubSyncProjectItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":{"node":{"items":{"nodes":[
			{"id":"PVTI_1","updatedAt":"2026-06-01T00:00:00Z","content":{"__typename":"Issue","url":"https://github.com/acme/api/issues/3"}},
			{"id":"PVTI_2","updatedAt":"2026-06-02T00:00:00Z","content":{"__typename":"DraftIssue","title":"draft"}}
		],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`))
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	var got []Artifact
	done, err := g.SyncUnit(context.Background(), srv.Client(),
		Unit{ID: "PVT_1", Kind: "github_project", Name: "Roadmap"}, "",
		func(_ context.Context, a Artifact) error { got = append(got, a); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("a fully-fetched board should report done")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 project items, got %d", len(got))
	}
	if got[0].ExternalID != "PVTI_1" || got[0].SourceNativeKind != "github_project_item" {
		t.Fatalf("unexpected artifact: %+v", got[0])
	}
	// A Projects v2 board is cursorless: every emit carries an empty Cursor.
	if got[0].Cursor != "" {
		t.Fatalf("project items are cursorless, got cursor %q", got[0].Cursor)
	}
	if got[0].Container.Kind != "github_project" {
		t.Fatalf("unexpected container: %+v", got[0].Container)
	}
}

func TestGitHubSyncUnitSkipsPullRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/api/issues" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") == "1" {
			// one real issue, one pull request (the issues endpoint returns both)
			_, _ = w.Write([]byte(`[
				{"number":7,"html_url":"https://github.com/acme/api/issues/7","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-02T12:00:00Z"},
				{"number":8,"html_url":"https://github.com/acme/api/pull/8","created_at":"2026-05-02T00:00:00Z","updated_at":"2026-06-03T12:00:00Z","pull_request":{"url":"x"}}
			]`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	var got []Artifact
	done, err := g.SyncUnit(context.Background(), srv.Client(), Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "", func(_ context.Context, a Artifact) error {
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
		t.Fatalf("expected 1 issue (PR skipped), got %d", len(got))
	}
	a := got[0]
	if a.ExternalID != "acme/api#7" || a.SourceNativeKind != "github_issue" {
		t.Fatalf("unexpected artifact: %+v", a)
	}
	if a.Container.ID != "acme/api" {
		t.Fatalf("unexpected container: %+v", a.Container)
	}
	// The item's cursor is its updated_at (not the PR's, which is skipped) — the emit
	// layer advances the unit cursor to it atomically with the manifest (ADR-0038).
	if a.Cursor != "2026-06-02T12:00:00Z" {
		t.Fatalf("unexpected item cursor: %q", a.Cursor)
	}
}

// A deleted repo (404) surfaces as an error so the per-unit drain retries/DLQs
// alone (ADR-0029) — it must not be swallowed.
func TestGitHubSyncUnit404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	_, err := g.SyncUnit(context.Background(), srv.Client(), Unit{ID: "acme/legacy"}, "", func(context.Context, Artifact) error { return nil })
	if err == nil {
		t.Fatal("expected an error for a deleted repo")
	}
}
