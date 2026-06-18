package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestGitHubConcurrentCommentsPreserveOrder locks the L3 throughput fix: a page's
// comments are fetched concurrently, but artifacts are still EMITTED in ascending
// updatedAt order with each art.Cursor monotonic — the high-water-mark contract
// (ADR-0031/0038) survives the fan-out. We also assert overlap actually happens
// (concurrent, not serialized) by tracking max in-flight comment requests.
func TestGitHubConcurrentCommentsPreserveOrder(t *testing.T) {
	const n = 30
	var issues []string
	for i := 0; i < n; i++ {
		// updatedAt strictly ascending with the page order.
		issues = append(issues, fmt.Sprintf(
			`{"number":%d,"html_url":"h%d","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-01T00:%02d:00Z","comments":1}`,
			i+1, i+1, i))
	}
	issuesJSON := "[" + strings.Join(issues, ",") + "]"

	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	hold := make(chan struct{}) // gate to force overlap
	var once sync.Once

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/api/issues" {
			_, _ = w.Write([]byte(issuesJSON))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/comments") {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			cur := inFlight
			mu.Unlock()
			// Once enough requests pile up concurrently, release the gate so they all
			// proceed; proves the fetches overlap rather than running one-by-one.
			if cur >= ghCommentConcurrency {
				once.Do(func() { close(hold) })
			}
			<-hold
			mu.Lock()
			inFlight--
			mu.Unlock()
			// derive a stable id from the issue number in the path
			parts := strings.Split(r.URL.Path, "/")
			num := parts[len(parts)-2]
			_, _ = w.Write([]byte(`[{"id":` + num + `,"body":"c"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	var emitted []string // ExternalID in emit order
	var cursors []string
	collect := func(_ context.Context, a Artifact) error {
		emitted = append(emitted, a.ExternalID)
		cursors = append(cursors, a.Cursor)
		return nil
	}
	if _, err := g.SyncUnit(context.Background(), srv.Client(),
		Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "", collect); err != nil {
		t.Fatalf("SyncUnit: %v", err)
	}

	if len(emitted) != n {
		t.Fatalf("emitted %d artifacts, want %d", len(emitted), n)
	}
	// Emit order is the page order (ascending updatedAt) and cursors are monotonic.
	for i := 0; i < n; i++ {
		want := fmt.Sprintf("acme/api#%d", i+1)
		if emitted[i] != want {
			t.Fatalf("emit[%d]=%q, want %q (order not preserved)", i, emitted[i], want)
		}
		if i > 0 && cursors[i] <= cursors[i-1] {
			t.Fatalf("cursor not ascending at %d: %q after %q", i, cursors[i], cursors[i-1])
		}
	}
	if maxInFlight < 2 {
		t.Fatalf("comment fetches did not overlap (max in-flight=%d) — not concurrent", maxInFlight)
	}
	if maxInFlight > ghCommentConcurrency {
		t.Fatalf("concurrency unbounded: max in-flight=%d > cap %d", maxInFlight, ghCommentConcurrency)
	}
}

// TestGitHubConcurrentCommentsDegradeAndPropagate locks that the concurrent path
// keeps the per-issue error policy: a terminal per-issue comment error (404) yields
// an issue-without-comments artifact and the rest still emit in order; a transient
// error (5xx) propagates and fails the whole page (the repo retries).
func TestGitHubConcurrentCommentsDegradeAndPropagate(t *testing.T) {
	page := `[
		{"number":1,"html_url":"h1","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-01T00:01:00Z","comments":1},
		{"number":2,"html_url":"h2","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-01T00:02:00Z","comments":1},
		{"number":3,"html_url":"h3","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-01T00:03:00Z","comments":1}
	]`

	// Degrade case: issue 2 is a 404 (deleted between list and fetch).
	t.Run("degrade", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/api/issues":
				_, _ = w.Write([]byte(page))
			case "/repos/acme/api/issues/2/comments":
				w.WriteHeader(http.StatusNotFound)
			default:
				if strings.HasSuffix(r.URL.Path, "/comments") {
					_, _ = w.Write([]byte(`[{"id":1,"body":"ok"}]`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		g := &GitHub{apiBaseURL: srv.URL}
		got := map[string][]byte{}
		var order []string
		collect := func(_ context.Context, a Artifact) error {
			got[a.ExternalID] = a.Raw
			order = append(order, a.ExternalID)
			return nil
		}
		if _, err := g.SyncUnit(context.Background(), srv.Client(),
			Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "", collect); err != nil {
			t.Fatalf("one degraded issue must not fail the repo: %v", err)
		}
		if len(order) != 3 || order[0] != "acme/api#1" || order[1] != "acme/api#2" || order[2] != "acme/api#3" {
			t.Fatalf("order not preserved through degradation: %v", order)
		}
		var doc struct {
			Comments []json.RawMessage `json:"comments"`
		}
		_ = json.Unmarshal(got["acme/api#2"], &doc)
		if len(doc.Comments) != 0 {
			t.Fatalf("issue 2 should degrade to zero comments, got %d", len(doc.Comments))
		}
		_ = json.Unmarshal(got["acme/api#1"], &doc)
		if len(doc.Comments) != 1 {
			t.Fatalf("issue 1 comments should be intact, got %d", len(doc.Comments))
		}
	})

	// Propagate case: issue 2 is a transient 500 → the whole page fails (repo retries).
	t.Run("propagate", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repos/acme/api/issues":
				_, _ = w.Write([]byte(page))
			case "/repos/acme/api/issues/2/comments":
				w.WriteHeader(http.StatusInternalServerError)
			default:
				if strings.HasSuffix(r.URL.Path, "/comments") {
					_, _ = w.Write([]byte(`[{"id":1,"body":"ok"}]`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		g := &GitHub{apiBaseURL: srv.URL}
		collect := func(_ context.Context, _ Artifact) error { return nil }
		_, err := g.SyncUnit(context.Background(), srv.Client(),
			Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "", collect)
		if err == nil {
			t.Fatal("a transient comment error must propagate so the repo retries")
		}
	})
}

// TestGitHubEstimateTotals locks the GraphQL batching shape and totalCount parsing:
// many repos batch into one aliased query (r0/r1/...), issues.totalCount lands per
// unit.ID, and a sub-batch GraphQL error is skipped (unknown), never failing.
func TestGitHubEstimateTotals(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		calls++
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Build a data object keyed by every alias present in the query.
		data := map[string]any{}
		// repo aliases r0.. carry an "issues.totalCount"; project aliases p0.. carry
		// "items.totalCount". Count is derived from the alias index for assertability.
		for i := 0; i < 60; i++ {
			ra := "r" + strconv.Itoa(i)
			if strings.Contains(req.Query, ra+": repository") {
				data[ra] = map[string]any{"issues": map[string]any{"totalCount": (i + 1) * 10}}
			}
			pa := "p" + strconv.Itoa(i)
			if strings.Contains(req.Query, pa+": node") {
				data[pa] = map[string]any{"items": map[string]any{"totalCount": (i + 1) * 100}}
			}
		}
		out, _ := json.Marshal(map[string]any{"data": data})
		_, _ = w.Write(out)
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}

	// 30 repos → 2 batches at ghEstimateBatchSize=25; alias index resets per batch.
	var units []Unit
	for i := 0; i < 30; i++ {
		units = append(units, Unit{ID: fmt.Sprintf("acme/repo%d", i), Kind: "github_repo"})
	}
	units = append(units, Unit{ID: "PVT_proj0", Kind: "github_project"})

	totals, err := g.EstimateTotals(context.Background(), srv.Client(), units)
	if err != nil {
		t.Fatalf("EstimateTotals: %v", err)
	}
	// repo0 is alias r0 in batch 1 → (0+1)*10 = 10.
	if totals["acme/repo0"] != 10 {
		t.Fatalf("acme/repo0 total=%d, want 10", totals["acme/repo0"])
	}
	// repo25 is the first of batch 2 → alias r0 again → 10.
	if totals["acme/repo25"] != 10 {
		t.Fatalf("acme/repo25 (batch-2 r0) total=%d, want 10", totals["acme/repo25"])
	}
	// repo24 is r24 of batch 1 → (24+1)*10 = 250.
	if totals["acme/repo24"] != 250 {
		t.Fatalf("acme/repo24 total=%d, want 250", totals["acme/repo24"])
	}
	// project p0 → (0+1)*100 = 100.
	if totals["PVT_proj0"] != 100 {
		t.Fatalf("PVT_proj0 total=%d, want 100", totals["PVT_proj0"])
	}
	// 30 repos = 2 batches + 1 project batch = 3 GraphQL calls.
	if calls != 3 {
		t.Fatalf("expected 3 batched GraphQL calls (2 repo + 1 project), got %d", calls)
	}
}

// TestGitHubEstimateTotalsBestEffort locks that a GraphQL sub-batch error leaves
// those units unknown (absent) without failing the whole estimate.
func TestGitHubEstimateTotalsBestEffort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return a GraphQL-level error.
		_, _ = w.Write([]byte(`{"errors":[{"message":"insufficient scopes"}]}`))
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	totals, err := g.EstimateTotals(context.Background(), srv.Client(),
		[]Unit{{ID: "acme/api", Kind: "github_repo"}})
	if err != nil {
		t.Fatalf("best-effort estimate must not return an error, got %v", err)
	}
	if _, ok := totals["acme/api"]; ok {
		t.Fatalf("a failed sub-batch must leave the unit unknown, got %v", totals)
	}
}

// TestGoogleDocsEstimateTotalsEmpty locks the gdocs no-cheap-total behavior.
func TestGoogleDocsEstimateTotalsEmpty(t *testing.T) {
	g := NewGoogleDocs()
	totals, err := g.EstimateTotals(context.Background(), nil,
		[]Unit{{ID: "drive", Kind: "google_drive"}})
	if err != nil {
		t.Fatalf("EstimateTotals: %v", err)
	}
	if len(totals) != 0 {
		t.Fatalf("gdocs EstimateTotals should be empty (unknown), got %v", totals)
	}
}
