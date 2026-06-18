package activity

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"integrations/internal/connectors"
	"integrations/internal/database"
	"integrations/internal/database/gen"
)

// reposServer is an httptest GitHub that serves /installation/repositories with a
// fixed repo set on page 1 and an empty page after, so DiscoverUnits sees exactly that
// set. Projects v2 discovery (best-effort) 404s and is skipped — repo discovery stands.
// reposFn lets a test vary the set between passes (to simulate a degraded discovery).
func reposServer(t *testing.T, reposFn func() []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/installation/repositories") {
			w.WriteHeader(http.StatusNotFound) // Projects v2 etc. — best-effort, skipped
			return
		}
		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`{"repositories":[]}`))
			return
		}
		var b strings.Builder
		b.WriteString(`{"repositories":[`)
		for i, full := range reposFn() {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"full_name":%q,"owner":{"login":"acme","type":"Organization"}}`, full)
		}
		b.WriteString(`]}`)
		_, _ = w.Write([]byte(b.String()))
	}))
}

// pointGitHubAt aims the registry's GitHub connector at the test source.
func pointGitHubAt(acts *Activities, url string) {
	acts.reg = connectors.NewRegistry(connectors.NewGitHubWithBaseURL(url), connectors.NewGoogleDocs())
}

func liveUnits(t *testing.T, store *database.Store, connID string) int64 {
	t.Helper()
	n, err := store.Q.CountLiveUnits(context.Background(), connID)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestDiscoverUnitsHealthySweeps locks the happy path: a first pass discovers the units;
// a second pass dropping ONE repo sweeps exactly that unit (status -> removed) — a
// normal discovery does sweep.
func TestDiscoverUnitsHealthySweeps(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()

	repos := []string{"acme/a", "acme/b", "acme/c"}
	src := reposServer(t, func() []string { return repos })
	defer src.Close()
	pointGitHubAt(acts, src.URL)

	// Pass 1: discover all three. No prior live units → not degraded, nothing to sweep.
	r1, err := acts.DiscoverUnits(ctx, DiscoverInput{ConnectionID: connID})
	if err != nil {
		t.Fatalf("pass1: %v", err)
	}
	if r1.Count != 3 || r1.Swept != 0 || r1.Degraded {
		t.Fatalf("pass1 unexpected: %+v", r1)
	}
	if n := liveUnits(t, store, connID); n != 3 {
		t.Fatalf("expected 3 live units, got %d", n)
	}

	// Pass 2: acme/c disappears (2 of 3 = above the 0.5 threshold → healthy) → sweep it.
	repos = []string{"acme/a", "acme/b"}
	r2, err := acts.DiscoverUnits(ctx, DiscoverInput{ConnectionID: connID})
	if err != nil {
		t.Fatalf("pass2: %v", err)
	}
	if r2.Degraded {
		t.Fatalf("a 2-of-3 discovery is healthy, not degraded: %+v", r2)
	}
	if r2.Swept != 1 {
		t.Fatalf("pass2 should sweep exactly the vanished unit, swept=%d", r2.Swept)
	}
	gone, err := store.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: connID, ExternalID: "acme/c"})
	if err != nil {
		t.Fatal(err)
	}
	if gone.Status != "removed" {
		t.Fatalf("the unseen unit must be tombstoned, status=%q", gone.Status)
	}
	if n := liveUnits(t, store, connID); n != 2 {
		t.Fatalf("expected 2 live units after sweep, got %d", n)
	}
}

// TestDiscoverUnitsDegradedDoesNotSweep is the critical safety test (ADR-0037): a pass
// returning 0 units, and a pass returning far fewer than the last known good, must NOT
// sweep — a transient blip can never tombstone the corpus.
func TestDiscoverUnitsDegradedDoesNotSweep(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()

	repos := []string{"acme/a", "acme/b", "acme/c", "acme/d"}
	src := reposServer(t, func() []string { return repos })
	defer src.Close()
	pointGitHubAt(acts, src.URL)

	if _, err := acts.DiscoverUnits(ctx, DiscoverInput{ConnectionID: connID}); err != nil {
		t.Fatalf("seed pass: %v", err)
	}
	if n := liveUnits(t, store, connID); n != 4 {
		t.Fatalf("expected 4 live units, got %d", n)
	}

	// Empty discovery (a token blip): degraded, no sweep, all 4 units survive.
	repos = []string{}
	rEmpty, err := acts.DiscoverUnits(ctx, DiscoverInput{ConnectionID: connID})
	if err != nil {
		t.Fatalf("empty pass: %v", err)
	}
	if !rEmpty.Degraded || rEmpty.Swept != 0 {
		t.Fatalf("an empty discovery must be degraded and sweep nothing: %+v", rEmpty)
	}
	if n := liveUnits(t, store, connID); n != 4 {
		t.Fatalf("empty discovery must NOT tombstone the corpus, live=%d", n)
	}

	// A suspicious drop: 1 of 4 (< 0.5 * 4) is degraded → no sweep.
	repos = []string{"acme/a"}
	rDrop, err := acts.DiscoverUnits(ctx, DiscoverInput{ConnectionID: connID})
	if err != nil {
		t.Fatalf("drop pass: %v", err)
	}
	if !rDrop.Degraded || rDrop.Swept != 0 {
		t.Fatalf("a 1-of-4 discovery must be degraded and sweep nothing: %+v", rDrop)
	}
	if n := liveUnits(t, store, connID); n != 4 {
		t.Fatalf("a suspicious drop must NOT sweep, live=%d", n)
	}
}
