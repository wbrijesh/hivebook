package connectors

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestGitHubSyncDrainsUpdatedAtPlateau locks the P1-6 fix: when more issues share
// one updatedAt than fit in the per-run page budget, the cursor must still advance
// past them. GitHub's `since` is inclusive and can't sub-filter a timestamp, so a
// naive "stop at the budget" would re-fetch the same head forever and never reach
// newer issues. The connector keeps paging through the plateau (design-doc 0011).
func TestGitHubSyncDrainsUpdatedAtPlateau(t *testing.T) {
	type iss struct {
		n       int
		updated string
	}
	// A 5-issue plateau at T0 (bigger than the 2x2 = 4-item budget below), then a
	// newer issue at T1 the cursor must reach.
	all := []iss{
		{1, "2026-06-01T00:00:00Z"}, {2, "2026-06-01T00:00:00Z"},
		{3, "2026-06-01T00:00:00Z"}, {4, "2026-06-01T00:00:00Z"},
		{5, "2026-06-01T00:00:00Z"}, {6, "2026-06-02T00:00:00Z"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/api/issues" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		since := q.Get("since")
		perPage, _ := strconv.Atoi(q.Get("per_page"))
		page, _ := strconv.Atoi(q.Get("page"))

		var filtered []iss
		for _, it := range all {
			if since == "" || it.updated >= since { // mirror GitHub's inclusive `since`
				filtered = append(filtered, it)
			}
		}
		start := (page - 1) * perPage
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + perPage
		if end > len(filtered) {
			end = len(filtered)
		}
		var b strings.Builder
		b.WriteString("[")
		for i, it := range filtered[start:end] {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"number":%d,"html_url":"https://github.com/acme/api/issues/%d","created_at":"%s","updated_at":"%s"}`,
				it.n, it.n, it.updated, it.updated)
		}
		b.WriteString("]")
		_, _ = w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL, perPage: 2, maxPages: 2} // budget = 4 items < plateau
	emitted := map[string]bool{}
	// Mirror the emit layer: the unit cursor is the high-water of committed items'
	// Artifact.Cursor (ADR-0038), so the test tracks the max cursor it emits.
	var cursor string
	collect := func(_ context.Context, a Artifact) error {
		emitted[a.ExternalID] = true
		if a.Cursor > cursor {
			cursor = a.Cursor
		}
		return nil
	}
	container := Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}

	if _, err := g.SyncUnit(context.Background(), srv.Client(), container, "", collect); err != nil {
		t.Fatalf("run1: %v", err)
	}
	pos1 := cursor
	if _, err := g.SyncUnit(context.Background(), srv.Client(), container, pos1, collect); err != nil {
		t.Fatalf("run2: %v", err)
	}
	pos2 := cursor

	// The cursor must clear the plateau and reach the newer issue — not stall at T0.
	if pos2 != "2026-06-02T00:00:00Z" {
		t.Fatalf("cursor should advance past the plateau, got %q (pos1=%q)", pos2, pos1)
	}
	for n := 1; n <= 6; n++ {
		id := fmt.Sprintf("acme/api#%d", n)
		if !emitted[id] {
			t.Fatalf("issue %s never emitted — the plateau stalled the cursor", id)
		}
	}
}
