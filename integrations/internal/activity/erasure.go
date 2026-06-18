package activity

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"

	"integrations/internal/database/gen"
	"integrations/internal/metrics"
)

// Erasure deletion outbox (ADR-0039) and the removed-unit GC (ADR-0037). These are the
// activities the MaintenanceWorkflow drives on a periodic loop. The invariant: the
// manifest row is NEVER dropped before its object delete is confirmed — a failed delete
// stays purge_pending for the next pass, so the pointer to un-erased bytes is never lost.

// Erasure tunables (constants-first, ADR-0011).
const (
	// purgeSLADays is the contractual GDPR erasure window: a purge_pending artifact that
	// has not confirmed deletion within this many days of the purge request (deleted_at)
	// is an SLA breach and PAGES — it does not sit quietly in the retry loop (ADR-0039).
	// Distinct from purgeGraceDays (the recovery window before bytes are deleted at all).
	purgeSLADays = 30

	// purgeBatch caps how many artifacts one PurgeArtifacts call processes — the work is
	// drained across calls by the workflow (loop until empty), so no single activity owns
	// an unbounded delete stream.
	purgeBatch = 200

	// slaScanLimit caps the SLA report so a large breach set doesn't return unbounded.
	slaScanLimit = 500
)

// --- PurgeArtifacts ----------------------------------------------------------

// PurgeInput bounds one purge pass.
type PurgeInput struct {
	// Limit caps the artifacts processed this call; <=0 uses purgeBatch.
	Limit int
}

// PurgeResult reports one purge pass: Deleted = bytes confirmed gone and manifest rows
// dropped; Failed = storage deletes that errored (rows kept purge_pending for retry);
// Orphaned = rows with no object key (nothing to delete — dropped directly).
type PurgeResult struct {
	Deleted  int
	Failed   int
	Orphaned int
}

// PurgeArtifacts drains the erasure outbox (ADR-0039): it lists purge_pending artifacts
// whose grace deadline has elapsed, deletes each object (a not-found counts as deleted —
// storage.Delete swallows it), and ONLY on confirmed deletion drops the manifest row. A
// storage delete that fails leaves the row purge_pending for the next pass — the outbox
// invariant: never drop the pointer to un-erased bytes. The deletes are collected and the
// rows dropped in one ConfirmArtifactDeletes batch.
func (a *Activities) PurgeArtifacts(ctx context.Context, in PurgeInput) (PurgeResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = purgeBatch
	}

	rows, err := a.store.Q.ListPurgePendingArtifacts(ctx, int32(limit))
	if err != nil {
		return PurgeResult{}, err
	}

	var res PurgeResult
	confirmed := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.ObjectKey == "" {
			// No bytes to erase (a manifest row that never got an object key) — the row
			// itself is the only thing to drop; do it directly, it's not "failed".
			confirmed = append(confirmed, r.ID)
			res.Orphaned++
			continue
		}
		if err := a.storage.Delete(ctx, r.ObjectKey); err != nil {
			// Transient/throttle: keep the row purge_pending for the next pass. Do NOT
			// drop it — that would orphan the bytes it claims to have purged (the v0.1 bug).
			metrics.PurgeDeletes.WithLabelValues("failed").Inc()
			res.Failed++
			continue
		}
		confirmed = append(confirmed, r.ID)
		res.Deleted++
		metrics.PurgeDeletes.WithLabelValues("deleted").Inc()
	}

	if len(confirmed) > 0 {
		if _, err := a.store.Q.ConfirmArtifactDeletes(ctx, confirmed); err != nil {
			return res, err
		}
	}
	if res.Orphaned > 0 {
		metrics.PurgeDeletes.WithLabelValues("orphaned").Add(float64(res.Orphaned))
	}
	return res, nil
}

// --- CheckPurgeSLA -----------------------------------------------------------

// SLAResult reports purge_pending artifacts that have blown the GDPR erasure SLA.
type SLAResult struct {
	Breached int
	IDs      []string
}

// CheckPurgeSLA finds purge_pending artifacts whose purge was requested more than
// purgeSLADays ago and are STILL not deleted — a stuck delete past the contractual
// window. It returns them and emits a page-worthy signal (the SLA gauge + an error-level
// log): "a stuck delete must page, not sit quietly" (ADR-0039). It does NOT delete or
// mutate — purge is PurgeArtifacts' job; this is the alarm.
func (a *Activities) CheckPurgeSLA(ctx context.Context) (SLAResult, error) {
	cutoff := nullTimeVal(time.Now().Add(-purgeSLADays * 24 * time.Hour))
	rows, err := a.store.Q.ListPurgePendingPastDeadline(ctx, gen.ListPurgePendingPastDeadlineParams{
		SlaCutoff: cutoff,
		RowLimit:  slaScanLimit,
	})
	if err != nil {
		return SLAResult{}, err
	}

	metrics.PurgeSLABreaches.Set(float64(len(rows)))
	if len(rows) == 0 {
		return SLAResult{}, nil
	}

	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	// Error-level so it pages: a purge stuck past the GDPR window is an incident. The
	// gauge above is the alert series; this is the diagnosable lineage. activity.GetLogger
	// is a no-op outside a real activity context, so the activity stays unit-testable.
	if activity.IsActivity(ctx) {
		activity.GetLogger(ctx).Error("purge_sla_breach", "count", len(ids))
	}
	return SLAResult{Breached: len(ids), IDs: ids}, nil
}

// --- MarkConnectionPurge -----------------------------------------------------

// MarkPurgeInput identifies a connection whose bytes must be erased immediately.
type MarkPurgeInput struct {
	ConnectionID string
	TenantID     string
}

// MarkConnectionPurge marks ALL a connection's artifacts purge_pending with an immediate
// deadline (now, no grace) — the disconnect erasure path (ADR-0039): the user revoked the
// connection, so its bytes are erased at once. Tenant-scoped so a connection id alone can
// never enqueue another tenant's bytes. The Phase-5 server Disconnect handler calls this;
// the actual delete is the outbox's (PurgeArtifacts) on the next maintenance pass.
func (a *Activities) MarkConnectionPurge(ctx context.Context, in MarkPurgeInput) error {
	return a.store.Q.MarkConnectionArtifactsPurgePending(ctx, gen.MarkConnectionArtifactsPurgePendingParams{
		ConnectionID: in.ConnectionID,
		TenantID:     in.TenantID,
	})
}

// --- GCRemovedUnits ----------------------------------------------------------

// GCResult reports how many tombstoned units were hard-deleted across all connections.
type GCResult struct {
	Deleted int
}

// GCRemovedUnits hard-deletes tombstoned (removed) sync_units past the grace cutoff,
// across every connection that has them (ADR-0037). It runs AFTER PurgeArtifacts so a
// removed unit's bytes are erased before its row is dropped. The grace cutoff is the same
// purgeGraceDays the sweep stamps the artifact deadline with, so a unit and its bytes age
// out together. Returns the total count for observability.
func (a *Activities) GCRemovedUnits(ctx context.Context) (GCResult, error) {
	cutoff := time.Now().Add(-purgeGraceDays * 24 * time.Hour)
	connIDs, err := a.store.Q.ListConnectionsWithRemovedUnits(ctx, cutoff)
	if err != nil {
		return GCResult{}, err
	}
	var total int
	for _, id := range connIDs {
		n, err := a.store.Q.GCRemovedUnits(ctx, gen.GCRemovedUnitsParams{
			ConnectionID: id,
			Cutoff:       cutoff,
		})
		if err != nil {
			return GCResult{Deleted: total}, err
		}
		total += int(n)
	}
	return GCResult{Deleted: total}, nil
}
