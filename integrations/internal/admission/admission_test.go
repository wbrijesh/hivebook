package admission

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"integrations/internal/connectors"
	"integrations/internal/database"
)

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

// newLimiter builds a Limiter over a fresh store; each test uses unique scope keys so
// the shared container's rows never leak across tests.
func newLimiter(t *testing.T) (*Limiter, func()) {
	t.Helper()
	store, err := database.New(testCfg)
	if err != nil {
		t.Fatal(err)
	}
	return New(store.Q), func() { _ = store.Close() }
}

// seed is the connector seed used in the tests: start at 2, hard cap at 4.
var seed = connectors.RateBudget{StartConcurrency: 2, MaxConcurrency: 4}

// TestAcquireBlocksAtLimitAndReleaseFrees locks the core CAS: acquire takes slots up to
// the seeded limit, the over-limit acquire fails (ok=false, no error), and a release
// frees exactly one slot back.
func TestAcquireBlocksAtLimitAndReleaseFrees(t *testing.T) {
	l, done := newLimiter(t)
	defer done()
	ctx := context.Background()
	key := "conn-" + t.Name()

	// Two slots (StartConcurrency=2): both acquire.
	for i := 0; i < 2; i++ {
		ok, err := l.Acquire(ctx, ScopeCredential, key, "github", seed)
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("acquire %d should succeed under the limit", i)
		}
	}
	// Third acquire is over the limit → blocked.
	ok, err := l.Acquire(ctx, ScopeCredential, key, "github", seed)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("acquire over the limit must be refused")
	}
	// Release frees a slot; the next acquire succeeds.
	if err := l.Release(ctx, ScopeCredential, key); err != nil {
		t.Fatal(err)
	}
	ok, err = l.Acquire(ctx, ScopeCredential, key, "github", seed)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("acquire after a release must succeed")
	}
}

// TestThrottleHalvesLimitAndSetsCooldown locks multiplicative decrease + cooldown: a
// throttle halves L and parks the bucket so an acquire is refused until the cooldown
// passes.
func TestThrottleHalvesLimitAndSetsCooldown(t *testing.T) {
	l, done := newLimiter(t)
	defer done()
	ctx := context.Background()
	key := "conn-" + t.Name()

	// Seed the bucket (L=2) via an acquire+release so in_flight is 0.
	if _, err := l.Acquire(ctx, ScopeCredential, key, "github", seed); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(ctx, ScopeCredential, key); err != nil {
		t.Fatal(err)
	}

	// Throttle with a short cooldown: L 2 → 1, cooldown armed.
	if err := l.RecordThrottle(ctx, ScopeCredential, key, 600*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.CurrentLimit(ctx, ScopeCredential, key); got != 1 {
		t.Fatalf("throttle should halve L to 1, got %d", got)
	}
	// Cooldown blocks the acquire even though in_flight (0) < limit (1).
	ok, err := l.Acquire(ctx, ScopeCredential, key, "github", seed)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("acquire during cooldown must be refused")
	}
	// After the cooldown passes, the acquire succeeds.
	time.Sleep(700 * time.Millisecond)
	ok, err = l.Acquire(ctx, ScopeCredential, key, "github", seed)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("acquire after the cooldown passes must succeed")
	}
}

// TestSuccessGrowsLimitUpToMax locks additive increase: sustained success grows L by one
// per recorded success while in_flight is near the limit, and never past the hard cap.
func TestSuccessGrowsLimitUpToMax(t *testing.T) {
	l, done := newLimiter(t)
	defer done()
	ctx := context.Background()
	key := "conn-" + t.Name()

	// Seed (L=2). Hold both slots so in_flight is at the limit — additive increase is
	// gated on near-the-ceiling load, so we must keep the bucket busy to grow it.
	for i := 0; i < 2; i++ {
		if ok, err := l.Acquire(ctx, ScopeCredential, key, "github", seed); err != nil || !ok {
			t.Fatalf("seed acquire %d: ok=%v err=%v", i, ok, err)
		}
	}
	// L: 2 → 3 → 4 (cap), then stays at 4.
	for i := 0; i < 5; i++ {
		if err := l.RecordSuccess(ctx, ScopeCredential, key, seed.MaxConcurrency); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := l.CurrentLimit(ctx, ScopeCredential, key); got != seed.MaxConcurrency {
		t.Fatalf("L should grow to the hard cap %d, got %d", seed.MaxConcurrency, got)
	}
}

// TestSuccessDoesNotGrowWhenIdle locks the gate: a success recorded while the bucket is
// idle (in_flight well below the limit) must NOT inflate L — growth tracks real load.
func TestSuccessDoesNotGrowWhenIdle(t *testing.T) {
	l, done := newLimiter(t)
	defer done()
	ctx := context.Background()
	key := "conn-" + t.Name()

	if _, err := l.Acquire(ctx, ScopeCredential, key, "github", seed); err != nil {
		t.Fatal(err)
	}
	if err := l.Release(ctx, ScopeCredential, key); err != nil { // in_flight back to 0
		t.Fatal(err)
	}
	if err := l.RecordSuccess(ctx, ScopeCredential, key, seed.MaxConcurrency); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.CurrentLimit(ctx, ScopeCredential, key); got != seed.StartConcurrency {
		t.Fatalf("L must not grow on an idle success, got %d", got)
	}
}

// TestCurrentLimitUnseeded locks the macro's seed-on-read contract: an unseeded bucket
// reports limit 0 without erroring, so the Capacity activity floors it.
func TestCurrentLimitUnseeded(t *testing.T) {
	l, done := newLimiter(t)
	defer done()
	got, err := l.CurrentLimit(context.Background(), ScopeTenant, "tenant-"+t.Name())
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("an unseeded bucket should report 0, got %d", got)
	}
}
