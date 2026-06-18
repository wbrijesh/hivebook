package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGitHubSyncFoldsIssueComments locks P2-1: an issue's artifact carries its
// comment thread (composite {issue, comments}), comments are fetched only when the
// issue has any, and a comment-less issue triggers no extra request.
func TestGitHubSyncFoldsIssueComments(t *testing.T) {
	commentHits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/api/issues":
			_, _ = w.Write([]byte(`[
				{"number":7,"html_url":"h7","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-02T00:00:00Z","comments":2},
				{"number":8,"html_url":"h8","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-03T00:00:00Z","comments":0}
			]`))
		case "/repos/acme/api/issues/7/comments":
			commentHits["7"]++
			_, _ = w.Write([]byte(`[{"id":1,"body":"first"},{"id":2,"body":"second"}]`))
		case "/repos/acme/api/issues/8/comments":
			commentHits["8"]++
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL} // perPage/maxPages default via the zero-guard
	got := map[string][]byte{}
	collect := func(_ context.Context, a Artifact) error { got[a.ExternalID] = a.Raw; return nil }
	if _, err := g.SyncUnit(context.Background(), srv.Client(),
		Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "", collect); err != nil {
		t.Fatalf("SyncUnit: %v", err)
	}

	var doc struct {
		Issue    json.RawMessage   `json:"issue"`
		Comments []json.RawMessage `json:"comments"`
	}
	// #7: the thread (issue + its two comments) is folded into one artifact.
	if err := json.Unmarshal(got["acme/api#7"], &doc); err != nil {
		t.Fatalf("issue 7 raw not a composite: %v", err)
	}
	if len(doc.Issue) == 0 {
		t.Fatal("issue 7 composite must retain the issue object")
	}
	if len(doc.Comments) != 2 {
		t.Fatalf("issue 7 should fold 2 comments, got %d", len(doc.Comments))
	}
	if commentHits["7"] != 1 {
		t.Fatalf("issue 7 comments fetched %d times (want 1)", commentHits["7"])
	}

	// #8: zero comments → empty list AND no comments request.
	if err := json.Unmarshal(got["acme/api#8"], &doc); err != nil {
		t.Fatalf("issue 8 raw not a composite: %v", err)
	}
	if len(doc.Comments) != 0 {
		t.Fatalf("issue 8 should have no comments, got %d", len(doc.Comments))
	}
	if commentHits["8"] != 0 {
		t.Fatalf("comment-less issue 8 must not trigger a comments fetch (hits=%d)", commentHits["8"])
	}
}

// TestGitHubSyncDegradesOnCommentError locks M1: a per-issue comment failure (e.g.
// a 404 from an issue deleted between the list and the fetch) must NOT fail the
// whole repo — the issue is emitted without comments and the rest of the page
// proceeds, so one bad issue can't DLQ the container (ADR-0029 isolation).
func TestGitHubSyncDegradesOnCommentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/api/issues":
			_, _ = w.Write([]byte(`[
				{"number":7,"html_url":"h7","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-02T00:00:00Z","comments":1},
				{"number":8,"html_url":"h8","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-03T00:00:00Z","comments":1}
			]`))
		case "/repos/acme/api/issues/7/comments":
			w.WriteHeader(http.StatusNotFound) // issue vanished between list and fetch
		case "/repos/acme/api/issues/8/comments":
			_, _ = w.Write([]byte(`[{"id":9,"body":"ok"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	got := map[string][]byte{}
	var cursor string
	collect := func(_ context.Context, a Artifact) error {
		got[a.ExternalID] = a.Raw
		if a.Cursor > cursor {
			cursor = a.Cursor
		}
		return nil
	}
	_, err := g.SyncUnit(context.Background(), srv.Client(),
		Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "", collect)
	if err != nil {
		t.Fatalf("a single bad issue must not fail the repo: %v", err)
	}

	var doc struct {
		Comments []json.RawMessage `json:"comments"`
	}
	// #7 degraded: still emitted, just without its comments.
	if _, ok := got["acme/api#7"]; !ok {
		t.Fatal("issue 7 must still be emitted despite its comment fetch failing")
	}
	if err := json.Unmarshal(got["acme/api#7"], &doc); err != nil {
		t.Fatalf("issue 7 raw: %v", err)
	}
	if len(doc.Comments) != 0 {
		t.Fatalf("issue 7 should degrade to zero comments, got %d", len(doc.Comments))
	}
	// #8 unaffected.
	if err := json.Unmarshal(got["acme/api#8"], &doc); err != nil {
		t.Fatalf("issue 8 raw: %v", err)
	}
	if len(doc.Comments) != 1 {
		t.Fatalf("issue 8 comments should be intact, got %d", len(doc.Comments))
	}
	// The cursor advanced past both — no DLQ stall on the bad issue.
	if cursor != "2026-06-03T00:00:00Z" {
		t.Fatalf("cursor should advance past both issues, got %q", cursor)
	}
}
