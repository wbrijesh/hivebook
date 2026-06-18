package activity

import (
	"context"
	"time"

	"integrations/internal/database/gen"
)

// --- StartRun ----------------------------------------------------------------

// RunInput identifies the connection a fresh run is starting for.
type RunInput struct {
	ConnectionID string
}

// startRunSQL re-arms prior-cycle terminal units for a fresh run: a unit that
// reached 'synced'/'error' last cycle is reset to 'idle' so this run re-checks it
// for deltas (its cursor is the high-water mark, so a re-check is cheap — only the
// tail since the cursor is re-pulled). committed_at is deliberately NOT touched:
// visibility persists across runs (ADR-0033). Raw SQL mirrors NextUnitBatch so the
// data-plane phase doesn't regenerate the query package.
//
// TODO: sqlc-ify — the equivalent generated query can replace this without changing
// the activity contract.
const startRunSQL = `
UPDATE sync_unit
SET status = 'idle', updated_at = now()
WHERE connection_id = $1
  AND selected = true
  AND status IN ('synced', 'error')`

// startRunConnectionSQL flips the connection to 'syncing' at the head of a run so the
// WatchConnections stream reflects the in-progress state immediately (the settle step
// sets the resting status at the end). Guarded so it never clobbers a sticky/terminal
// state ('needs_reauth', 'disabled') or a stop-in-progress ('stopping') — those win
// over a fresh run.
const startRunConnectionSQL = `
UPDATE connector_connection
SET status = 'syncing', updated_at = now()
WHERE id = $1
  AND status NOT IN ('needs_reauth', 'disabled', 'stopping')`

// StartRun resets prior-cycle terminal units so a fresh run re-checks them for
// deltas, and flips the connection to 'syncing' so the UI shows the run live. Only
// called at the head of a fresh run (input.MidRun == false in the workflow) — a
// continueAsNew resume must NOT reset mid-run progress (design-doc 0012). Idempotent:
// re-running it over an already-reset/already-syncing connection is a no-op.
func (a *Activities) StartRun(ctx context.Context, in RunInput) error {
	if _, err := a.store.SQLDB().ExecContext(ctx, startRunSQL, in.ConnectionID); err != nil {
		return err
	}
	conn, err := a.store.Q.GetConnectionByID(ctx, in.ConnectionID)
	if err != nil {
		return err
	}
	if _, err := a.store.SQLDB().ExecContext(ctx, startRunConnectionSQL, in.ConnectionID); err != nil {
		return err
	}
	// Wake the tenant's WatchConnections stream so "syncing" surfaces at once (best-
	// effort — NOTIFY is a push hint, not a correctness gate).
	_ = a.store.NotifyConnectionChanged(ctx, conn.TenantID)
	return nil
}

// --- SettleConnection --------------------------------------------------------

// SettleInput identifies the connection to settle after a drain (or a stop).
type SettleInput struct {
	ConnectionID string
}

// SettleResult reports the resting state the connection was settled to, plus the
// unit tallies it was derived from (for the workflow's logging/metrics).
type SettleResult struct {
	// Status is the rolled-up connection status:
	// 'backfilling' | 'connected' | 'connected_warnings' | 'needs_reauth' | 'error'.
	Status      string
	Total       int // selected units
	Errored     int // selected units in 'error'
	Synced      int // selected units in 'synced'
	Uncommitted int // selected units never committed once (committed_at IS NULL)
}

// settleCountsSQL tallies the selected units for the rollup: total, errored, synced,
// and "uncommitted" (committed_at IS NULL — never finished a full pass). Tombstoned
// ('removed') units are excluded: they're leaving the corpus, not part of the live
// status. Raw SQL (TODO: sqlc-ify) for the same reason as the other run-control queries.
const settleCountsSQL = `
SELECT
  count(*) FILTER (WHERE selected AND status <> 'removed'),
  count(*) FILTER (WHERE selected AND status = 'error'),
  count(*) FILTER (WHERE selected AND status = 'synced'),
  count(*) FILTER (WHERE selected AND status <> 'removed' AND committed_at IS NULL)
FROM sync_unit
WHERE connection_id = $1`

// SettleConnection folds the selected-unit statuses crossed with the run PHASE into the
// connection's resting status (design-doc 0012, "Visibility and status"):
//
//   - needs_reauth — sticky: an auth failure already set it; settle never papers over it.
//   - error        — every selected unit failed (all-failed).
//   - backfilling  — at least one selected unit has never committed once (committed_at
//     IS NULL) and is still making progress (not all of the uncommitted are errored): the
//     initial backfill is in flight, so the connection is neither error nor fully connected.
//   - connected_warnings — every selected unit has committed at least once, but some are
//     in 'error' (a poison unit backing off): visible + flagged.
//   - connected    — every selected unit committed and none errored.
//
// Progressive visibility (ADR-0033) is unaffected: per-unit committed_at still gates each
// unit's artifacts; this rollup is only the connection-level summary the UI shows.
func (a *Activities) SettleConnection(ctx context.Context, in SettleInput) (SettleResult, error) {
	var total, errored, synced, uncommitted int
	if err := a.store.SQLDB().QueryRowContext(ctx, settleCountsSQL, in.ConnectionID).
		Scan(&total, &errored, &synced, &uncommitted); err != nil {
		return SettleResult{}, err
	}

	status := settleStatus(total, errored, uncommitted)

	conn, err := a.store.Q.GetConnectionByID(ctx, in.ConnectionID)
	if err != nil {
		return SettleResult{}, err
	}

	if err := a.setConnectionSettled(ctx, conn, status, errored); err != nil {
		return SettleResult{}, err
	}

	return SettleResult{Status: status, Total: total, Errored: errored, Synced: synced, Uncommitted: uncommitted}, nil
}

// settleStatus is the pure rollup so it is unit-testable without a DB. Order matters:
// all-failed beats backfilling (a connection whose every unit errored is 'error', not
// "still backfilling"); an in-flight backfill (some uncommitted, not all errored) beats
// connected; warnings beats plain connected.
func settleStatus(total, errored, uncommitted int) string {
	switch {
	case total == 0:
		return "connected" // nothing selected — a resting, healthy connection
	case errored == total:
		return "error"
	case uncommitted > errored:
		// Some selected unit has never committed and isn't merely an errored one — the
		// initial backfill is still in flight.
		return "backfilling"
	case errored > 0:
		return "connected_warnings"
	default:
		return "connected"
	}
}

// setConnectionSettledSQL settles the connection row: the resting status, last_success_at
// advanced only when every unit has committed at least once (status connected /
// connected_warnings — a backfill in progress has NOT had a successful full pass), and
// last_error cleared on any non-error settle. Raw SQL (TODO: sqlc-ify).
const setConnectionSettledSQL = `
UPDATE connector_connection
SET status = $2,
    last_success_at = CASE WHEN $2 IN ('connected', 'connected_warnings') THEN now() ELSE last_success_at END,
    last_error = CASE WHEN $2 = 'error' THEN last_error ELSE '' END,
    updated_at = now()
WHERE id = $1`

func (a *Activities) setConnectionSettled(ctx context.Context, conn gen.ConnectorConnection, status string, errored int) error {
	// needs_reauth is sticky: an auth failure already set it (classify), and a settle
	// must not paper over it — the connection needs the user to reconnect.
	if conn.Status == "needs_reauth" {
		return nil
	}
	if _, err := a.store.SQLDB().ExecContext(ctx, setConnectionSettledSQL, conn.ID, status); err != nil {
		return err
	}
	// Wake the tenant's WatchConnections/WatchProjects streams: the resting status just
	// changed (design-doc 0012, "live status"). Best-effort — NOTIFY is a push hint, not
	// a correctness gate; the next subscribe re-reads regardless.
	_ = a.store.NotifyConnectionChanged(ctx, conn.TenantID)
	// A connection-level lineage row summarising the settle (artifacts left 0 — the
	// per-unit rows carry the counts).
	result := "success"
	switch {
	case status == "error":
		result = "error"
	case status == "backfilling":
		result = "backfilling"
	case errored > 0:
		result = "partial"
	}
	_ = a.store.Q.InsertSyncRun(ctx, gen.InsertSyncRunParams{
		TenantID:     conn.TenantID,
		ConnectionID: conn.ID,
		ConnectorID:  conn.ConnectorID,
		UnitID:       "", // whole-connection summary
		Result:       result,
		Artifacts:    0,
		Error:        "",
		StartedAt:    time.Now(),
	})
	return nil
}
