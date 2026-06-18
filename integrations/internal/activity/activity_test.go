package activity

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"integrations/internal/connectors"
	"integrations/internal/database"
	"integrations/internal/database/gen"
	"integrations/internal/oauth"
	"integrations/internal/storage"
)

// testCfg points at the container started in TestMain.
var testCfg database.Config

func TestMain(m *testing.M) {
	dbName, dbUser, dbPwd := "database", "user", "password"
	c, err := postgres.Run(
		context.Background(),
		"postgres:17-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPwd),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		log.Fatalf("start postgres: %v", err)
	}
	host, _ := c.Host(context.Background())
	port, _ := c.MappedPort(context.Background(), "5432/tcp")
	testCfg = database.Config{Host: host, Port: port.Port(), User: dbUser, Password: dbPwd, Database: dbName}

	code := m.Run()
	_ = c.Terminate(context.Background())
	os.Exit(code)
}

// fakeS3 is a minimal in-memory S3 endpoint: it answers the HeadBucket/CreateBucket/
// PutObject calls storage.Put makes and records every object body, so a test can
// assert which artifacts were written (and that a deduped item is NOT re-written).
type fakeS3 struct {
	mu   sync.Mutex
	puts map[string]int // object key -> times written
}

func newFakeS3() (*fakeS3, *httptest.Server) {
	f := &fakeS3{puts: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead: // HeadBucket
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			// CreateBucket (no object) vs PutObject (path has an object segment).
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
			if len(parts) < 2 || parts[1] == "" {
				w.WriteHeader(http.StatusOK) // CreateBucket
				return
			}
			f.mu.Lock()
			f.puts[r.URL.Path]++
			f.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	return f, srv
}

func (f *fakeS3) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.puts {
		n += c
	}
	return n
}

// newCipher builds a 32-byte-keyed cipher for the tests.
func newCipher(t *testing.T) *oauth.Cipher {
	t.Helper()
	c, err := oauth.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// setup spins a fresh Store + fake storage + Activities for one test, seeding a
// GitHub connection whose access token is a static bearer (no installation).
func setup(t *testing.T) (*Activities, *database.Store, *fakeS3, string) {
	t.Helper()
	store, err := database.New(testCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	fake, s3srv := newFakeS3()
	t.Cleanup(s3srv.Close)
	st, err := storage.New(context.Background(), storage.Config{
		Endpoint: s3srv.URL, Region: "us-east-1", AccessKey: "k", SecretKey: "s", Env: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	cipher := newCipher(t)
	access, err := cipher.EncryptString("tok")
	if err != nil {
		t.Fatal(err)
	}
	// A unique tenant/external_id per test so the shared Postgres container's rows
	// (connections + manifest dedup state) never leak across tests.
	uniq := strings.ReplaceAll(t.Name(), "/", "_")
	conn, err := store.Q.UpsertConnection(context.Background(), gen.UpsertConnectionParams{
		TenantID: "tenant-" + uniq, Region: "us", ConnectorID: "github", Account: "acme",
		AccessToken: access, ExternalID: "acme-" + uniq,
	})
	if err != nil {
		t.Fatal(err)
	}

	reg := connectors.NewRegistry(connectors.NewGitHub(), connectors.NewGoogleDocs())
	acts := New(store, st, reg, cipher, map[string]connectors.Creds{})
	return acts, store, fake, conn.ID
}

// issuesServer is an httptest GitHub returning a fixed set of issues (ascending
// updatedAt), each with no comments. A second arm serves an empty page so paging ends.
func issuesServer(t *testing.T, issues []struct {
	n       int
	updated string
}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/issues") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte("[]"))
			return
		}
		var b strings.Builder
		b.WriteString("[")
		for i, it := range issues {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"number":%d,"html_url":"h%d","created_at":"2026-01-01T00:00:00Z","updated_at":"%s","comments":0}`, it.n, it.n, it.updated)
		}
		b.WriteString("]")
		_, _ = w.Write([]byte(b.String()))
	}))
}

// pointConnectorAt aims the registry's GitHub connector at the test source.
func pointConnectorAt(acts *Activities, url string) {
	acts.reg = connectors.NewRegistry(connectors.NewGitHubWithBaseURL(url), connectors.NewGoogleDocs())
}

func seedUnit(t *testing.T, store *database.Store, connID string) {
	t.Helper()
	if err := store.Q.UpsertSyncUnit(context.Background(), gen.UpsertSyncUnitParams{
		ConnectionID: connID, ExternalID: "acme/api", Name: "acme/api", Kind: "github_repo", Generation: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestSyncUnitWritesCommitsAndAdvancesCursor locks the ADR-0038 happy path: each
// changed artifact is written to storage and committed with the cursor advanced to
// its updatedAt, the unit is committed (committed_at set) and marked synced, and
// the result reports the right counts.
func TestSyncUnitWritesCommitsAndAdvancesCursor(t *testing.T) {
	acts, store, fake, connID := setup(t)
	src := issuesServer(t, []struct {
		n       int
		updated string
	}{
		{1, "2026-06-01T00:00:00Z"}, {2, "2026-06-02T00:00:00Z"}, {3, "2026-06-03T00:00:00Z"},
	})
	defer src.Close()
	pointConnectorAt(acts, src.URL)
	seedUnit(t, store, connID)

	res, err := acts.SyncUnit(context.Background(), SyncInput{
		ConnectionID: connID, TenantID: "tenant-1", ConnectorID: "github", UnitID: "acme/api",
	})
	if err != nil {
		t.Fatalf("SyncUnit: %v", err)
	}
	if !res.Done || res.Artifacts != 3 || res.Status != "synced" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if fake.total() != 3 {
		t.Fatalf("expected 3 objects written, got %d", fake.total())
	}
	unit, err := store.Q.GetSyncUnit(context.Background(), gen.GetSyncUnitParams{ConnectionID: connID, ExternalID: "acme/api"})
	if err != nil {
		t.Fatal(err)
	}
	if unit.Cursor != "2026-06-03T00:00:00Z" {
		t.Fatalf("cursor should be the latest committed updatedAt, got %q", unit.Cursor)
	}
	if !unit.CommittedAt.Valid {
		t.Fatal("committed_at must be stamped on a full pass (ADR-0033)")
	}
	if unit.Status != "synced" {
		t.Fatalf("unit status should be synced, got %q", unit.Status)
	}
}

// TestSyncUnitDedupSkipsRewriteButAdvancesCursor locks the dedup arm: a re-run with
// identical content writes no new object (content-hash dedup) yet still completes,
// and the cursor remains at the high-water mark.
func TestSyncUnitDedupSkipsRewriteButAdvancesCursor(t *testing.T) {
	acts, store, fake, connID := setup(t)
	issues := []struct {
		n       int
		updated string
	}{
		{1, "2026-06-01T00:00:00Z"}, {2, "2026-06-02T00:00:00Z"},
	}
	src := issuesServer(t, issues)
	defer src.Close()
	pointConnectorAt(acts, src.URL)
	seedUnit(t, store, connID)

	in := SyncInput{ConnectionID: connID, TenantID: "tenant-1", ConnectorID: "github", UnitID: "acme/api"}
	if _, err := acts.SyncUnit(context.Background(), in); err != nil {
		t.Fatalf("run1: %v", err)
	}
	if fake.total() != 2 {
		t.Fatalf("run1 should write 2 objects, got %d", fake.total())
	}
	res, err := acts.SyncUnit(context.Background(), in)
	if err != nil {
		t.Fatalf("run2: %v", err)
	}
	// Identical content → dedup → no manifest written this run, no new object.
	if res.Artifacts != 0 {
		t.Fatalf("run2 should write 0 artifacts (all deduped), got %d", res.Artifacts)
	}
	if fake.total() != 2 {
		t.Fatalf("run2 must not re-write objects, total=%d", fake.total())
	}
	unit, _ := store.Q.GetSyncUnit(context.Background(), gen.GetSyncUnitParams{ConnectionID: connID, ExternalID: "acme/api"})
	if unit.Cursor != "2026-06-02T00:00:00Z" {
		t.Fatalf("cursor should hold the high-water mark across a deduped run, got %q", unit.Cursor)
	}
}

// TestNextUnitBatch locks the drain selection: selected, non-terminal, non-backed-off
// units are returned; a synced unit and a future-retry unit are excluded.
func TestNextUnitBatch(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()
	for _, id := range []string{"u-idle", "u-synced", "u-backoff"} {
		if err := store.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{
			ConnectionID: connID, ExternalID: id, Name: id, Kind: "github_repo", Generation: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Units now default to not-selected (migration 000005); NextUnitBatch only drains
	// selected units. Select all three so the assertions below verify the status and
	// backoff filters exclude the synced/backoff ones even when they ARE selected.
	if err := store.Q.SetUnitSelection(ctx, gen.SetUnitSelectionParams{
		ConnectionID: connID, SelectedIds: []string{"u-idle", "u-synced", "u-backoff"},
	}); err != nil {
		t.Fatal(err)
	}
	_ = store.Q.SetUnitStatus(ctx, gen.SetUnitStatusParams{Status: "synced", ConnectionID: connID, ExternalID: "u-synced"})
	_ = store.Q.SetUnitRetry(ctx, gen.SetUnitRetryParams{
		LastError: "boom", NextRetryAt: nullTimeVal(time.Now().Add(time.Hour)),
		ConnectionID: connID, ExternalID: "u-backoff",
	})

	res, err := acts.NextUnitBatch(ctx, BatchInput{ConnectionID: connID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, id := range res.UnitIDs {
		got[id] = true
	}
	if !got["u-idle"] {
		t.Fatal("an idle selected unit must be in the batch")
	}
	if got["u-synced"] {
		t.Fatal("a synced unit must not be re-drained this cycle")
	}
	if got["u-backoff"] {
		t.Fatal("a unit in poison backoff must be excluded until next_retry_at")
	}
}
