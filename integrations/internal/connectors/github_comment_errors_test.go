package connectors

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"integrations/internal/metrics"
)

// TestGitHubSyncPropagatesConnectionWideCommentErrors locks the safety-critical arm
// of M1: a connection-wide failure during a comment fetch (revoked credentials, or
// a rate limit) must STOP the whole run — not silently degrade to issue-only
// artifacts (which would swallow a needs_reauth). Complements the 404 degrade test.
func TestGitHubSyncPropagatesConnectionWideCommentErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		isAuth bool
	}{
		{"auth", http.StatusUnauthorized, true},
		{"ratelimit", http.StatusTooManyRequests, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/acme/api/issues":
					_, _ = w.Write([]byte(`[{"number":7,"html_url":"h7","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-02T00:00:00Z","comments":1}]`))
				case "/repos/acme/api/issues/7/comments":
					if tc.status == http.StatusTooManyRequests {
						w.Header().Set("Retry-After", "1")
					}
					w.WriteHeader(tc.status)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			g := &GitHub{apiBaseURL: srv.URL}
			emitted := map[string]bool{}
			_, err := g.SyncUnit(context.Background(), srv.Client(),
				Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "",
				func(_ context.Context, a Artifact) error { emitted[a.ExternalID] = true; return nil })

			if err == nil {
				t.Fatalf("a %s during a comment fetch must stop the run", tc.name)
			}
			if tc.isAuth {
				if _, ok := AsAuth(err); !ok {
					t.Fatalf("want an AuthError, got %v", err)
				}
			} else if _, ok := AsRateLimit(err); !ok {
				t.Fatalf("want a RateLimitError, got %v", err)
			}
			if emitted["acme/api#7"] {
				t.Fatal("issue 7 must NOT be committed comment-less on a connection-wide error")
			}
		})
	}
}

// TestGitHubSyncPropagatesTransientCommentError locks the P3-5 stricter end-state:
// a transient comment-fetch failure (a 5xx) must propagate so River retries the
// repo and preserves the thread — NOT degrade (which would stickily drop the
// high-value comments). Contrast with the 404 terminal case, which degrades.
func TestGitHubSyncPropagatesTransientCommentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/api/issues":
			_, _ = w.Write([]byte(`[{"number":7,"html_url":"h7","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-02T00:00:00Z","comments":1}]`))
		case "/repos/acme/api/issues/7/comments":
			w.WriteHeader(http.StatusServiceUnavailable) // transient
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	g := &GitHub{apiBaseURL: srv.URL}
	emitted := map[string]bool{}
	_, err := g.SyncUnit(context.Background(), srv.Client(),
		Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "",
		func(_ context.Context, a Artifact) error { emitted[a.ExternalID] = true; return nil })

	if err == nil {
		t.Fatal("a transient 5xx during a comment fetch must propagate (retry the repo), not degrade")
	}
	if emitted["acme/api#7"] {
		t.Fatal("issue 7 must not be degrade-emitted on a transient comment error")
	}
}

// TestGitHubSyncStopPropagatesDuringCommentFetch locks #1: a Stop (parent-context
// cancellation) landing during a comment fetch must propagate as cancellation —
// not be misread as a per-issue comment failure. It must not degrade-emit the
// issue and must not tick the comment-dropped metric (no false telemetry on Stop).
func TestGitHubSyncStopPropagatesDuringCommentFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/api/issues":
			_, _ = w.Write([]byte(`[{"number":7,"html_url":"h7","created_at":"2026-05-01T00:00:00Z","updated_at":"2026-06-02T00:00:00Z","comments":1}]`))
			cancel() // a Stop lands right after the list is delivered, before comments
		case "/repos/acme/api/issues/7/comments":
			_, _ = w.Write([]byte(`[{"id":1,"body":"x"}]`)) // would succeed, but ctx is cancelled
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	dropped := metrics.ConnectorCapHits.WithLabelValues("github", "issue_comments_dropped")
	before := testutil.ToFloat64(dropped)

	g := &GitHub{apiBaseURL: srv.URL}
	emitted := map[string]bool{}
	_, err := g.SyncUnit(ctx, srv.Client(),
		Unit{ID: "acme/api", Kind: "github_repo", Name: "acme/api"}, "",
		func(_ context.Context, a Artifact) error { emitted[a.ExternalID] = true; return nil })

	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("a Stop during a comment fetch must propagate cancellation, got %v", err)
	}
	if emitted["acme/api#7"] {
		t.Fatal("issue 7 must not be degrade-emitted on a Stop")
	}
	if after := testutil.ToFloat64(dropped); after != before {
		t.Fatalf("a Stop must not tick the comment-dropped metric (false telemetry): before=%v after=%v", before, after)
	}
}
