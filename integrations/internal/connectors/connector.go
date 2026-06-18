// Package connectors holds the per-source drivers and the registry over them.
// A connector captures raw only — no normalization, no embeddings (ADR-0023);
// interpretation happens downstream over the raw corpus.
package connectors

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/oauth2"
)

// Container is the source sub-grouping an artifact came from — a Drive folder, a
// GitHub repo (design-doc 0008). It is the per-artifact *manifest* container
// recorded for filing priors, distinct from the sync Unit below: for Google Docs
// the manifest container is the parent folder while the sync unit is the whole
// Drive (ADR-0035/0029). Connector-agnostic in shape.
type Container struct {
	ID   string
	Kind string
	Name string
}

// Unit is a connector-defined *sync partition* — the thing the platform drains and
// advances a cursor for (a GitHub repo or Projects v2 board; a single "drive" for
// Google Docs). The count ranges from one to many per connection and is the source's
// real partition, not a platform-imposed granularity (ADR-0035).
type Unit struct {
	ID   string
	Kind string
	Name string
}

// RateBudget is the connector's *seed* for the adaptive admission controller: a
// starting concurrency guess plus hard caps the limiter never crosses. The real
// ceiling is learned at runtime via AIMD feedback — this is only the initial guess
// (ADR-0036). KnownReqPerHour is the published hourly cap, or 0 when none exists.
type RateBudget struct {
	StartConcurrency int
	MaxConcurrency   int
	KnownReqPerHour  int
}

// Artifact is one raw item a connector emits: the verbatim bytes plus the
// connector-agnostic manifest fields. Downstream never re-parses Raw to recover
// these.
type Artifact struct {
	SourceNativeKind string
	Container        Container
	ExternalID       string
	SourceURL        string
	SourceCreatedAt  *time.Time
	SourceUpdatedAt  *time.Time
	ContentType      string
	Raw              []byte
	// Cursor is this item's high-water value (e.g. its updatedAt/modifiedTime) —
	// the value the unit's cursor advances to once this item is committed, in the
	// same transaction as its manifest (ADR-0038). Empty for cursorless sources
	// (Projects v2 boards), where the unit re-fetches whole each run.
	Cursor string
}

// EmitFunc is called once per artifact during a sync; it persists the raw bytes
// and the manifest. A connector stops and returns the error if emit fails.
type EmitFunc func(context.Context, Artifact) error

// Connector is our driver for a source type.
type Connector interface {
	ID() string        // stable key: "gdocs", "github"
	Name() string      // display: "Google Docs"
	Archetype() string // "document", "record" (design-doc 0008)
	Scopes() []string  // OAuth scopes requested

	// OAuthConfig builds the provider config for the Authorization Code flow
	// (used to exchange the callback code for a token).
	OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config

	// AuthorizeURL is where Connect sends the user to grant access. Most connectors
	// return the OAuth consent URL; a GitHub App returns its install URL so the
	// user picks repos to grant in the same flow.
	AuthorizeURL(creds Creds, state, redirectURL string) string

	// Account returns a human label for the connected account (email / login),
	// captured when the connection is created.
	Account(ctx context.Context, hc *http.Client) (string, error)

	// DiscoverUnits lists the sync partitions for this connection — the unit each
	// SyncUnit drain owns and advances a cursor for (a GitHub repo; a single "drive"
	// for Google Docs, whose modifiedTime stream is one sequence). This is distinct
	// from the per-artifact manifest container recorded for filing priors (design-doc
	// 0008/0009/0010, ADR-0035).
	DiscoverUnits(ctx context.Context, hc *http.Client) ([]Unit, error)

	// SyncUnit fetches artifacts in one unit changed since cursor, calling emit per
	// artifact. The cursor is no longer returned: it lives in sync_unit and advances
	// per committed item via Artifact.Cursor, in the same transaction as that item's
	// manifest (ADR-0038), so an interruption re-does only the un-committed tail.
	// cursor is empty on the first (backfill) sync. Returns done=true when the unit's
	// stream was fully drained this call (caught up), false when it stopped at a
	// page/runtime budget with more to fetch.
	SyncUnit(ctx context.Context, hc *http.Client, unit Unit, cursor string, emit EmitFunc) (done bool, err error)

	// RateBudget seeds the adaptive admission controller — the starting concurrency
	// guess and hard caps; the platform learns the real ceiling at runtime (ADR-0036).
	RateBudget() RateBudget

	// EstimateTotals returns a best-effort estimate of how many items each unit will
	// sync (unit.ID → count), fetched cheaply up front so the platform can sort units
	// descending and show synced/total progress. Best-effort: a unit absent from the
	// map (or an empty map, or a partial result after a sub-batch error) means
	// "unknown" and must NOT fail discovery. Implementations should swallow per-batch
	// errors and return what they could estimate.
	EstimateTotals(ctx context.Context, hc *http.Client, units []Unit) (map[string]int, error)
}

// Registry is the set of available connectors, in a stable order.
type Registry struct {
	byID  map[string]Connector
	order []Connector
}

// NewRegistry builds a registry over the given connectors.
func NewRegistry(cs ...Connector) *Registry {
	r := &Registry{byID: make(map[string]Connector, len(cs))}
	for _, c := range cs {
		r.byID[c.ID()] = c
		r.order = append(r.order, c)
	}
	return r
}

// Get returns the connector with the given id.
func (r *Registry) Get(id string) (Connector, bool) {
	c, ok := r.byID[id]
	return c, ok
}

// All returns the connectors in registration order.
func (r *Registry) All() []Connector { return r.order }
