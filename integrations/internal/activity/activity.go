// Package activity holds the Temporal activities — the only place sync does I/O
// (source HTTP, Postgres, object storage, token refresh). Activities are
// idempotent and at-least-once; the workflow stays a payload-free control plane
// (design-doc 0012, ADR-0034).
package activity

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"integrations/internal/admission"
	"integrations/internal/connectors"
	"integrations/internal/database"
	"integrations/internal/database/gen"
	"integrations/internal/metrics"
	"integrations/internal/oauth"
	"integrations/internal/storage"
)

// notifyEvery throttles the live-progress NOTIFY during a drain: WatchProjects is woken
// once per this many NEW artifacts (plus once at the end of the call) so synced/total
// climbs visibly without a NOTIFY-per-item storm (design-doc 0012, "live progress").
const notifyEvery = 25

// Poison-unit backoff bounds (ADR-0036): a failed unit arms an exponential
// backoff so one dead repo doesn't burn a slot every pass. Phase 2a uses a fixed
// first-failure delay; the workflow tracks the attempt count and grows it later.
const poisonBackoff = 5 * time.Minute

// errType labels for the Temporal application errors the workflow branches on.
const (
	errTypeAuth      = "auth"      // non-retryable: needs_reauth, stop retrying
	errTypeRateLimit = "ratelimit" // retryable: throttle recorded, cooldown armed (ADR-0036)
	errTypeSync      = "sync"      // retryable: transient unit failure
)

// Activities groups the activity methods so the worker can register them as a set
// and they share dependencies. Each method is a Temporal activity — a plain func
// returning (result, error) — so it can be invoked from the workflow.
//
// TODO(0012 Phase 3/4): add the remaining activities (Checkpoint, ReadRateLimit,
// RefreshToken, Reconcile, Erase) and the mark-and-sweep prune.
type Activities struct {
	store   *database.Store
	storage *storage.Store
	reg     *connectors.Registry
	cipher  *oauth.Cipher
	creds   map[string]connectors.Creds
	limiter *admission.Limiter
}

// New constructs an Activities over its I/O dependencies.
func New(store *database.Store, storage *storage.Store, reg *connectors.Registry, cipher *oauth.Cipher, creds map[string]connectors.Creds) *Activities {
	return &Activities{
		store: store, storage: storage, reg: reg, cipher: cipher, creds: creds,
		limiter: admission.New(store.Q),
	}
}

// Admission-controller defaults (ADR-0036, constants-first ADR-0011). The credential
// budget is adaptive (seeded per-connector via RateBudget); these are the platform-wide
// constants the per-credential AIMD limit is bounded by.
const (
	// tenantInflightCap is the hard per-tenant in-flight cap — fairness so one whale
	// can't starve the pool (ADR-0036). Seeded into the tenant-scope bucket.
	tenantInflightCap = 16
	// globalWorkerCap bounds any single batch regardless of credential/tenant headroom.
	globalWorkerCap = 32
)

// Mark-and-sweep discovery tunables (ADR-0037, constants-first ADR-0011).
const (
	// degradedThreshold is the degraded-discovery guard: when the connection already
	// had live units, a pass that returns fewer than this fraction of them is treated
	// as a FAILED reconcile (no sweep, surfaced). A bad provider minute or an auth blip
	// returning 0/50-of-5000 units must never tombstone a tenant's corpus — the first
	// and most important line of defence (ADR-0037). An empty result is always degraded.
	degradedThreshold = 0.5

	// purgeGraceDays is the tombstone grace before a swept (removed) unit's artifact
	// BYTES are deleted — the second line of defence behind the guard (ADR-0037). The
	// removed unit's artifacts are marked purge_pending with purge_deadline = now +
	// this; the erasure outbox deletes them only after the deadline. Distinct from the
	// 30-day GDPR SLA in CheckPurgeSLA: this is the recovery window, that is the contract.
	purgeGraceDays = 7
)

// --- DiscoverUnits -----------------------------------------------------------

// DiscoverInput identifies the connection to discover units for. Generation is no
// longer passed in: DiscoverUnits picks its own monotonic generation (max+1 over the
// connection's units) so the sweep can tombstone exactly the units this pass did not
// re-observe. The field is kept for the workflow's existing call site; it is ignored.
type DiscoverInput struct {
	ConnectionID string
	// Deprecated: DiscoverUnits derives the generation itself (max+1). Ignored.
	Generation int64
}

// DiscoverResult reports the reconcile outcome: Count discovered this pass, Swept
// units tombstoned (0 on a degraded pass), and Degraded — the guard tripped, so the
// pass was NOT swept and the corpus is intact (ADR-0037).
type DiscoverResult struct {
	Count    int
	Swept    int
	Degraded bool
}

// DiscoverUnits is the mark-and-sweep reconcile (ADR-0037). It (1) reads the current
// live-unit count (last known good), (2) lists the source's units and upserts each
// stamping a fresh generation = max+1 + last_seen_at (the "mark"), (3) applies the
// degraded-discovery GUARD — an empty result, or a drop below degradedThreshold of the
// last known good, is a FAILED reconcile: no sweep, surfaced via last_error, Degraded
// returned true — so a transient blip never tombstones the corpus. Only a healthy pass
// (4) SWEEPS unseen units (generation < current → 'removed', a soft tombstone) and marks
// their artifacts purge_pending with a grace deadline (now + purgeGraceDays) for the
// erasure outbox (ADR-0039). The grace is the second line of defence behind the guard.
func (a *Activities) DiscoverUnits(ctx context.Context, in DiscoverInput) (DiscoverResult, error) {
	conn, connector, err := a.loadConnection(ctx, in.ConnectionID)
	if err != nil {
		return DiscoverResult{}, err
	}

	// (1) Last known good: how many live units the connection has before this pass.
	lastKnownGood, err := a.store.Q.CountLiveUnits(ctx, conn.ID)
	if err != nil {
		return DiscoverResult{}, err
	}

	// Fresh generation for this pass: max(generation)+1 over the connection's units, so
	// "generation < current" selects exactly the units this pass did NOT re-observe.
	maxGen, err := a.store.Q.MaxUnitGeneration(ctx, conn.ID)
	if err != nil {
		return DiscoverResult{}, err
	}
	generation := maxGen + 1

	client, err := a.clientFor(ctx, conn)
	if err != nil {
		return DiscoverResult{}, a.classify(ctx, conn, "", err)
	}
	units, err := connector.DiscoverUnits(ctx, client)
	if err != nil {
		return DiscoverResult{}, a.classify(ctx, conn, "", err)
	}

	// (2) Mark: upsert every observed unit with the fresh generation/last_seen_at.
	for _, u := range units {
		if err := a.store.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{
			ConnectionID: conn.ID,
			ExternalID:   u.ID,
			Name:         u.Name,
			Kind:         u.Kind,
			Generation:   generation,
		}); err != nil {
			return DiscoverResult{}, err
		}
	}

	// (3) Degraded-discovery guard (ADR-0037): empty, or a suspicious drop versus the
	// last known good, is a FAILED reconcile — do NOT sweep, surface a warning, return
	// Degraded. The marks above stand (a unit re-seen next healthy pass is fine); only
	// the destructive sweep is gated. A first discovery (lastKnownGood == 0) is exempt:
	// there is nothing to tombstone, so an initially-empty connection is not "degraded".
	discovered := len(units)
	degraded := discovered == 0 && lastKnownGood > 0
	if lastKnownGood > 0 && float64(discovered) < degradedThreshold*float64(lastKnownGood) {
		degraded = true
	}
	if degraded {
		metrics.DiscoveryDegraded.WithLabelValues(conn.ConnectorID).Inc()
		_ = a.store.Q.SetConnectionStatus(ctx, gen.SetConnectionStatusParams{
			Status:    conn.Status, // resting status unchanged; only surface the warning
			LastError: fmt.Sprintf("degraded discovery: saw %d of %d known units — sweep skipped", discovered, lastKnownGood),
			ID:        conn.ID,
		})
		_ = a.store.NotifyConnectionChanged(ctx, conn.TenantID)
		return DiscoverResult{Count: discovered, Swept: 0, Degraded: true}, nil
	}

	// (4) Sweep: tombstone unseen units, then mark their artifacts for graced purge.
	swept, err := a.store.Q.SweepUnseenUnits(ctx, gen.SweepUnseenUnitsParams{
		ConnectionID: conn.ID,
		Generation:   generation,
	})
	if err != nil {
		return DiscoverResult{}, err
	}
	if swept > 0 {
		deadline := nullTimeVal(time.Now().Add(purgeGraceDays * 24 * time.Hour))
		if err := a.store.Q.MarkRemovedUnitArtifactsPurgePending(ctx, gen.MarkRemovedUnitArtifactsPurgePendingParams{
			PurgeDeadline: deadline,
			ConnectionID:  conn.ID,
		}); err != nil {
			return DiscoverResult{}, err
		}
	}

	// (5) Best-effort totals: only on a healthy pass, ask the connector to estimate how
	// many items each unit will sync (unit.ID → count) so the UI can show synced/total and
	// sort descending by total. Strictly best-effort — an error, an empty map, or a unit
	// absent from the result just leaves total_estimate at its prior value (or 0); it must
	// NEVER fail discovery (ADR-0035, design-doc 0012). A stale estimate self-corrects next pass.
	a.estimateTotals(ctx, conn, connector, client, units)

	// Mark the catalog discovered: this is a healthy (non-degraded) pass, so the unit
	// list is real — NULL only before the first such pass. The UI reads NULL as the
	// post-connect cold-start ("discovering…") rather than a false "no projects"
	// (design-doc 0012). Re-stamped every healthy pass; the degraded guard above
	// returned early, so a degraded pass never sets it.
	if err := a.store.Q.SetCatalogDiscovered(ctx, conn.ID); err != nil {
		return DiscoverResult{}, err
	}

	// Wake the tenant's WatchProjects stream: the catalog was refreshed (new units to
	// pick, swept ones gone, totals estimated) — the discover-only path relies on this.
	_ = a.store.NotifyConnectionChanged(ctx, conn.TenantID)
	return DiscoverResult{Count: discovered, Swept: int(swept), Degraded: false}, nil
}

// estimateTotals stores the connector's best-effort per-unit item counts so the UI can
// show synced/total and sort by total. Best-effort in every dimension: a connector error,
// an empty/partial map, or a unit missing from the result simply leaves that unit's
// total_estimate untouched — it must never fail discovery (the caller ignores the outcome).
func (a *Activities) estimateTotals(ctx context.Context, conn gen.ConnectorConnection, connector connectors.Connector, client *http.Client, units []connectors.Unit) {
	totals, err := connector.EstimateTotals(ctx, client, units)
	if err != nil {
		// Swallow: a failed estimate is not a failed discovery. Totals stay at their
		// prior value (or 0); the next healthy pass re-estimates.
		slog.Warn("estimate_totals_failed", "error", err.Error(), "connection_id", conn.ID)
		return
	}
	for id, total := range totals {
		if err := a.store.Q.SetUnitTotal(ctx, gen.SetUnitTotalParams{
			ConnectionID:  conn.ID,
			ExternalID:    id,
			TotalEstimate: int32(total),
		}); err != nil {
			// One bad write doesn't sink the rest — keep going, surface nothing.
			slog.Warn("set_unit_total_failed", "error", err.Error(), "connection_id", conn.ID, "unit", id)
		}
	}
}

// --- NextUnitBatch -----------------------------------------------------------

// BatchInput asks for the next slice of units to drain.
type BatchInput struct {
	ConnectionID string
	Limit        int
}

// BatchResult is the bounded set of unit external_ids the workflow drains next —
// the list never enters Temporal history; it is re-derived from sync_unit each
// batch (design-doc 0012: the table is the resumption state).
type BatchResult struct {
	UnitIDs []string
}

// nextUnitBatchSQL selects the next slice of selected units that still need work
// this run: not yet terminal for the cycle ('synced'/'error'), with the poison
// backoff (next_retry_at) null or elapsed. It is a raw query (not sqlc-generated)
// so the data-plane phase doesn't have to regenerate the query package; the
// equivalent sqlc query can replace it later without changing the activity contract.
const nextUnitBatchSQL = `
SELECT external_id FROM sync_unit
WHERE connection_id = $1
  AND selected = true
  AND status NOT IN ('synced', 'error', 'removed')
  AND (next_retry_at IS NULL OR next_retry_at <= now())
ORDER BY external_id
LIMIT $2`

// NextUnitBatch returns up to Limit selected units that still need work this run.
// The workflow drains the table in bounded batches this way, never holding the
// unit list in Temporal history (design-doc 0012).
func (a *Activities) NextUnitBatch(ctx context.Context, in BatchInput) (BatchResult, error) {
	rows, err := a.store.SQLDB().QueryContext(ctx, nextUnitBatchSQL, in.ConnectionID, in.Limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return BatchResult{}, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	return BatchResult{UnitIDs: ids}, nil
}

// --- SyncUnit ----------------------------------------------------------------

// SyncInput identifies one unit to drain within a connection.
type SyncInput struct {
	ConnectionID string
	TenantID     string
	ConnectorID  string
	UnitID       string
}

// SyncResult reports a unit drain: Done = the stream was caught up this call,
// Artifacts = bytes-changed manifests written (after dedup), Status = the unit's
// resulting status.
type SyncResult struct {
	Done      bool
	Artifacts int
	Status    string
}

// SyncUnit drains one sync unit: it reads the unit's cursor, pulls artifacts since
// it, and — per artifact — writes the bytes to object storage FIRST (content-
// addressed, idempotent), THEN in a single Postgres transaction writes the manifest
// row and advances the cursor (ADR-0038). It heartbeats the cursor each item so a
// retry resumes from the last committed point and cancellation is delivered. A Stop
// (cancellation) returns the partial result without marking the unit errored — a
// stop is not a failure (ADR-0040). On done it stamps committed_at (ADR-0033) and
// 'synced'; connector errors classify into needs_reauth (non-retryable),
// rate-limit (retryable), or a poison-backed-off unit error.
func (a *Activities) SyncUnit(ctx context.Context, in SyncInput) (SyncResult, error) {
	started := time.Now()
	conn, connector, err := a.loadConnection(ctx, in.ConnectionID)
	if err != nil {
		return SyncResult{}, err
	}
	unit, err := a.store.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{
		ConnectionID: in.ConnectionID,
		ExternalID:   in.UnitID,
	})
	if err != nil {
		return SyncResult{}, err
	}

	if err := a.store.Q.SetUnitStatus(ctx, gen.SetUnitStatusParams{
		Status: "syncing", ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
	}); err != nil {
		return SyncResult{}, err
	}
	// Wake the tenant's WatchProjects stream: this unit just went 'syncing'.
	_ = a.store.NotifyConnectionChanged(ctx, in.TenantID)

	client, err := a.clientFor(ctx, conn)
	if err != nil {
		return SyncResult{}, a.classify(ctx, conn, in.UnitID, err)
	}

	// Micro admission (ADR-0036): take a credential slot before touching the source. A
	// refusal (!ok) means the limiter is at its learned ceiling or in a Retry-After
	// cooldown — return a retryable error so Temporal re-schedules the unit; the slot
	// frees as other units finish. We do NOT snooze-loop here (that would hold a worker).
	ok, err := a.limiter.Acquire(ctx, admission.ScopeCredential, in.ConnectionID, conn.ConnectorID, connector.RateBudget())
	if err != nil {
		return SyncResult{}, err
	}
	if !ok {
		return SyncResult{}, temporal.NewApplicationError("admission limit reached", errTypeRateLimit)
	}
	defer func() { _ = a.limiter.Release(ctx, admission.ScopeCredential, in.ConnectionID) }()

	var written int
	emit := a.makeEmit(in, conn, unit.Cursor, &written)

	done, syncErr := connector.SyncUnit(ctx, client, connectors.Unit{
		ID: unit.ExternalID, Kind: unit.Kind, Name: unit.Name,
	}, unit.Cursor, emit)

	// Final live-progress flush: surface the tail of the drain (the last < notifyEvery new
	// artifacts the throttle didn't NOTIFY) on every exit — caught up, stopped, or errored —
	// so synced/total lands its final value live. Only when this call actually wrote something.
	if written > 0 {
		_ = a.store.NotifyConnectionChanged(ctx, in.TenantID)
	}

	// A cancellation (Stop) is not a failure: the cursor is already persisted per
	// committed item, so return the partial progress with the unit left as-is
	// (ResetActiveUnits in the workflow's settle returns it to idle). ADR-0040.
	if ctx.Err() != nil {
		return SyncResult{Done: false, Artifacts: written, Status: "syncing"}, ctx.Err()
	}

	if syncErr != nil {
		// Throttle feedback (ADR-0036): a source rate-limit multiplicatively decreases the
		// credential limit and arms a Retry-After cooldown, so the next batch fans out
		// smaller and no unit acquires until the source recovers. classify then returns a
		// retryable error so this unit retries after the cooldown.
		if rl, ok := connectors.AsRateLimit(syncErr); ok {
			_ = a.limiter.RecordThrottle(ctx, admission.ScopeCredential, in.ConnectionID, rl.RetryAfter)
		}
		a.recordRun(ctx, in, runResult(syncErr), written, started, syncErr)
		return SyncResult{Artifacts: written, Status: "error"}, a.classify(ctx, conn, in.UnitID, syncErr)
	}

	// Success feedback (ADR-0036): a clean drain additively increases the credential limit
	// toward the connector's hard cap, so a healthy source discovers more headroom.
	_ = a.limiter.RecordSuccess(ctx, admission.ScopeCredential, in.ConnectionID, connector.RateBudget().MaxConcurrency)

	status := "syncing"
	if done {
		// Caught up: stamp committed_at for visibility (ADR-0033) and mark synced.
		if err := a.store.Q.CommitUnit(ctx, gen.CommitUnitParams{
			ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
		}); err != nil {
			return SyncResult{}, err
		}
		if err := a.store.Q.SetUnitStatus(ctx, gen.SetUnitStatusParams{
			Status: "synced", ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
		}); err != nil {
			return SyncResult{}, err
		}
		_ = a.store.Q.ClearUnitRetry(ctx, gen.ClearUnitRetryParams{
			ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
		})
		status = "synced"
		// Wake the tenant's WatchProjects stream: this unit committed + went 'synced'
		// (its artifacts are now visible — ADR-0033).
		_ = a.store.NotifyConnectionChanged(ctx, in.TenantID)
	}

	a.recordRun(ctx, in, "success", written, started, nil)
	return SyncResult{Done: done, Artifacts: written, Status: status}, nil
}

// makeEmit builds the per-artifact persistence closure that implements the
// ADR-0038 write order: storage first, then manifest + cursor in one transaction.
// high seeds the in-memory high-water mark with the unit's persisted cursor so the
// closure only ever advances the cursor forward, in the order items are emitted
// (ascending updatedAt → monotonic), without re-reading the row each item.
func (a *Activities) makeEmit(in SyncInput, conn gen.ConnectorConnection, high string, written *int) connectors.EmitFunc {
	return func(ctx context.Context, art connectors.Artifact) error {
		// Cooperative cancellation: stop cleanly at the artifact boundary; the cursor
		// is already committed up to the previous item (ADR-0040).
		if ctx.Err() != nil {
			return ctx.Err()
		}

		hash := contentHash(art.Raw)

		// (1) Dedup: an unchanged item is skipped (no storage write, no manifest), but
		// the cursor must still advance past it so the next run doesn't re-scan it.
		prev, err := a.store.Q.GetArtifactContentHash(ctx, gen.GetArtifactContentHashParams{
			ConnectionID: in.ConnectionID, ExternalID: art.ExternalID,
		})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && prev == hash {
			if art.Cursor > high {
				high = art.Cursor
				_ = a.store.Q.SetUnitCursor(ctx, gen.SetUnitCursorParams{
					Cursor: art.Cursor, ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
				})
			}
			heartbeat(ctx, art.Cursor)
			return nil
		}

		// (2) Changed: write the bytes FIRST (content-addressed, idempotent), THEN the
		// manifest row + cursor advance in ONE transaction (ADR-0038). A crash between
		// the Put and the commit leaves a harmless orphan object (re-written identically
		// next run) and an un-advanced cursor (the item is re-done, dedup absorbs it).
		objectKey, err := a.storage.Put(ctx, conn.ConnectorID, conn.Region, conn.TenantID,
			conn.ConnectorID+"/"+hash, art.ContentType, art.Raw)
		if err != nil {
			return err
		}

		tx, err := a.store.SQLDB().BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }() // no-op after a successful Commit
		q := a.store.Q.WithTx(tx)

		if err := q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
			TenantID:         conn.TenantID,
			Region:           conn.Region,
			ConnectionID:     in.ConnectionID,
			Source:           conn.ConnectorID,
			SourceNativeKind: art.SourceNativeKind,
			ContainerID:      art.Container.ID,
			SyncContainerID:  in.UnitID,
			ContainerKind:    art.Container.Kind,
			ContainerName:    art.Container.Name,
			ExternalID:       art.ExternalID,
			SourceUrl:        art.SourceURL,
			ObjectKey:        objectKey,
			ContentHash:      hash,
			SourceCreatedAt:  nullTime(art.SourceCreatedAt),
			SourceUpdatedAt:  nullTime(art.SourceUpdatedAt),
		}); err != nil {
			return err
		}
		// Advance the cursor in the SAME transaction — only forward, and only for a
		// cursored source (a cursorless unit emits Cursor==""). This is the atomic
		// manifest+cursor write of ADR-0038.
		advance := art.Cursor != "" && art.Cursor > high
		if advance {
			if err := q.SetUnitCursor(ctx, gen.SetUnitCursorParams{
				Cursor: art.Cursor, ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
			}); err != nil {
				return err
			}
		}
		// Bump the live synced count in the SAME transaction so progress is atomic with the
		// commit (ADR-0038). Only NEW writes increment (dedup-skips returned above); this is
		// a running estimate that CommitUnit reconciles to the exact manifest count at commit.
		if err := q.IncrementUnitArtifactCount(ctx, gen.IncrementUnitArtifactCountParams{
			Delta: 1, ConnectionID: in.ConnectionID, ExternalID: in.UnitID,
		}); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if advance {
			high = art.Cursor
		}

		*written++
		metrics.ArtifactsIngested.WithLabelValues(conn.ConnectorID).Inc()
		// Throttled live-progress NOTIFY: wake WatchProjects every notifyEvery new artifacts
		// so synced/total climbs visibly without a NOTIFY storm (the call NOTIFYs once more at
		// the end). The exact count is reconciled at commit; this is the running view.
		if *written%notifyEvery == 0 {
			_ = a.store.NotifyConnectionChanged(ctx, in.TenantID)
		}
		// Heartbeat the committed cursor so a retry resumes here and a Stop is delivered.
		heartbeat(ctx, art.Cursor)
		return nil
	}
}

// --- Capacity ----------------------------------------------------------------

// CapacityInput identifies the connection a batch is being sized for. No secrets — the
// limiter keys on ids only (ADR-0036).
type CapacityInput struct {
	ConnectionID string
	TenantID     string
	ConnectorID  string
}

// CapacityResult is the per-batch fan-out the workflow uses as the NextUnitBatch limit.
type CapacityResult struct {
	P int
}

// Capacity is the admission controller's macro level (ADR-0036): it returns the batch
// size P = min(credentialLimit, tenantCap, globalWorkerCap), with a floor of 1. The
// workflow calls it at the top of each drain batch so the fan-out tracks the limiter's
// learned ceiling — never I/O in deterministic workflow code, so this lives in an
// activity. The micro level (SyncUnit's per-unit Acquire) is the hard backstop; this is
// the cheap macro hint that keeps the batch from over-fanning in the first place.
func (a *Activities) Capacity(ctx context.Context, in CapacityInput) (CapacityResult, error) {
	connector, ok := a.reg.Get(in.ConnectorID)
	if !ok {
		return CapacityResult{}, fmt.Errorf("unknown connector %q", in.ConnectorID)
	}
	budget := connector.RateBudget()

	// Credential budget: seed-on-read so the very first batch reads the connector's
	// StartConcurrency rather than an empty 0.
	credLimit, err := a.limiter.CurrentLimit(ctx, admission.ScopeCredential, in.ConnectionID)
	if err != nil {
		return CapacityResult{}, err
	}
	if credLimit == 0 {
		credLimit = budget.StartConcurrency
	}

	// Tenant cap: the same limiter keyed by tenant, seeded from a platform constant.
	if err := a.limiter.SeedTenant(ctx, in.TenantID, tenantInflightCap); err != nil {
		return CapacityResult{}, err
	}
	tenantCap, err := a.limiter.CurrentLimit(ctx, admission.ScopeTenant, in.TenantID)
	if err != nil {
		return CapacityResult{}, err
	}

	p := min3(credLimit, tenantCap, globalWorkerCap)
	if p < 1 {
		p = 1 // floor: always make progress, one unit at a time if every dimension is 0
	}
	return CapacityResult{P: p}, nil
}

// min3 returns the least of three ints, ignoring non-positive values (an unseeded or
// floored dimension shouldn't clamp the batch to 0 — the caller floors at 1).
func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// --- GetSyncSchedule ---------------------------------------------------------

// ScheduleInput identifies the connection whose auto-sync cadence to read.
type ScheduleInput struct {
	ConnectionID string
}

// ScheduleResult is the per-connection auto-sync cadence: Enabled gates the periodic
// re-sync timer (off → run only on a manual trigger), IntervalSeconds is the cadence.
type ScheduleResult struct {
	Enabled         bool
	IntervalSeconds int32
}

// GetSyncSchedule reads the connection's user-configurable auto-sync cadence from
// Postgres. The ConnectionWorkflow calls it on every idle entry (a cheap control
// activity, never I/O in deterministic workflow code) so a change applies live and
// survives continueAsNew (design-doc 0012).
func (a *Activities) GetSyncSchedule(ctx context.Context, in ScheduleInput) (ScheduleResult, error) {
	row, err := a.store.Q.GetSyncSchedule(ctx, in.ConnectionID)
	if err != nil {
		return ScheduleResult{}, err
	}
	return ScheduleResult{Enabled: row.AutoSyncEnabled, IntervalSeconds: row.SyncIntervalSeconds}, nil
}

// --- helpers -----------------------------------------------------------------

// loadConnection loads a connection by id (cross-tenant; the worker runs
// server-side) and resolves its connector.
func (a *Activities) loadConnection(ctx context.Context, id string) (gen.ConnectorConnection, connectors.Connector, error) {
	conn, err := a.store.Q.GetConnectionByID(ctx, id)
	if err != nil {
		return gen.ConnectorConnection{}, nil, err
	}
	connector, ok := a.reg.Get(conn.ConnectorID)
	if !ok {
		return conn, nil, fmt.Errorf("unknown connector %q", conn.ConnectorID)
	}
	return conn, connector, nil
}

// classify turns a connector error into the right outcome + Temporal error:
//   - auth → mark the connection needs_reauth and return a NON-retryable error
//     (retrying revoked credentials is pointless — ADR-0040 / design-doc 0010).
//   - rate-limit → a retryable error; the workflow snoozes (Phase 3 wires Retry-After).
//   - anything else → arm the poison backoff on the unit and return a retryable error.
func (a *Activities) classify(ctx context.Context, conn gen.ConnectorConnection, unitID string, err error) error {
	if _, ok := connectors.AsAuth(err); ok {
		_ = a.store.Q.SetNeedsReauth(ctx, gen.SetNeedsReauthParams{LastError: err.Error(), ID: conn.ID})
		_ = a.store.NotifyConnectionChanged(ctx, conn.TenantID) // surface needs_reauth live
		return temporal.NewNonRetryableApplicationError("source rejected credentials", errTypeAuth, err)
	}
	if rl, ok := connectors.AsRateLimit(err); ok {
		metrics.SourceRateLimited.WithLabelValues(conn.ConnectorID).Inc()
		_ = rl // Phase 3: surface RetryAfter to the workflow as the snooze duration.
		return temporal.NewApplicationError("source rate limited", errTypeRateLimit, err)
	}
	if unitID != "" {
		_ = a.store.Q.SetUnitRetry(ctx, gen.SetUnitRetryParams{
			LastError:    err.Error(),
			NextRetryAt:  nullTimeVal(time.Now().Add(poisonBackoff)),
			ConnectionID: conn.ID,
			ExternalID:   unitID,
		})
		_ = a.store.NotifyConnectionChanged(ctx, conn.TenantID) // surface the unit error live
	}
	return temporal.NewApplicationError(err.Error(), errTypeSync, err)
}

// runResult maps a connector error to a sync_run result label.
func runResult(err error) string {
	switch {
	case isAuth(err):
		return "needs_reauth"
	case isRateLimit(err):
		return "rate_limited"
	default:
		return "error"
	}
}

func isAuth(err error) bool      { _, ok := connectors.AsAuth(err); return ok }
func isRateLimit(err error) bool { _, ok := connectors.AsRateLimit(err); return ok }

// recordRun writes the sync_run lineage row and folds the metrics for one drain.
func (a *Activities) recordRun(ctx context.Context, in SyncInput, result string, artifacts int, started time.Time, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	_ = a.store.Q.InsertSyncRun(ctx, gen.InsertSyncRunParams{
		TenantID:     in.TenantID,
		ConnectionID: in.ConnectionID,
		ConnectorID:  in.ConnectorID,
		UnitID:       in.UnitID,
		Result:       result,
		Artifacts:    int32(artifacts),
		Error:        msg,
		StartedAt:    started,
	})
	// Per-run outcome counts live in the sync_run row (the lineage), not a metric — a
	// per-connector counter by result would re-derive what the table already holds.
	// The duration is the mechanism series we keep (design-doc 0012, "Observability").
	metrics.SyncUnitDuration.WithLabelValues(in.ConnectorID).Observe(time.Since(started).Seconds())
}

// heartbeat records the cursor as the activity's heartbeat detail so a retry
// resumes from the last committed point and a Stop is delivered (ADR-0040). It is
// a no-op outside a real activity context (so the closure is unit-testable without
// a Temporal harness).
func heartbeat(ctx context.Context, cursor string) {
	if activity.IsActivity(ctx) {
		activity.RecordHeartbeat(ctx, cursor)
	}
}

// contentHash is the artifact's content-address — hex sha256 of the raw bytes.
func contentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil || t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func nullTimeVal(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}
