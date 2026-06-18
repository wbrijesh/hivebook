// Package workflow holds the Temporal workflows for sync orchestration. One
// long-lived ConnectionWorkflow per connection owns the lifecycle: schedule,
// trigger, auth state, retries, cancellation, and the sync driver (design-doc
// 0012, ADR-0034). Workflow state stays tiny and payload-free — the sync_unit
// table is the resumption state, not workflow history.
package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"integrations/internal/activity"
	igtemporal "integrations/internal/temporal"
)

// Activity names the workflow invokes. Activities are registered on the worker as a
// struct (RegisterActivity(acts)), so each registers under its bare method name; the
// workflow references them by that name (decoupled from the *Activities receiver, so
// the test env can mock them by name). These MUST match the method names in
// internal/activity.
const (
	actStartRun         = "StartRun"
	actDiscoverUnits    = "DiscoverUnits"
	actNextUnitBatch    = "NextUnitBatch"
	actSyncUnit         = "SyncUnit"
	actSettleConnection = "SettleConnection"
	actCapacity         = "Capacity"
	actGetSyncSchedule  = "GetSyncSchedule"
)

// Signal names the ConnectionWorkflow accepts. Exported so the integrations
// service (the Temporal client) and later workflow phases share one source of
// truth (design-doc 0012: the API translates RPCs into these signals).
const (
	// SignalTriggerSync requests an out-of-schedule sync; coalesced if one runs.
	SignalTriggerSync = "TriggerSync"
	// SignalStopSync cooperatively cancels the in-flight drain at a chunk boundary.
	SignalStopSync = "StopSync"
	// SignalDisconnect tears the connection down and ends the workflow.
	SignalDisconnect = "Disconnect"
	// SignalUpdateSelection changes which sources (repos/projects/channels) sync.
	SignalUpdateSelection = "UpdateSelection"
	// SignalRefreshCatalog runs a discover-only pass (no drain) so the sync_unit
	// catalog is populated/refreshed for selection before the first sync. The server
	// fires it (best-effort, fire-and-forget) on connect and on ListProjects.
	SignalRefreshCatalog = "RefreshCatalog"
	// SignalUpdateSchedule wakes the idle wait so it re-reads the auto-sync cadence
	// (on/off + interval) from Postgres and re-arms the timer. It carries no payload —
	// it is a "re-evaluate", not a run: the new schedule lives in the row, read by the
	// GetSyncSchedule activity on the next idle entry (design-doc 0012).
	SignalUpdateSchedule = "UpdateSchedule"
)

// Run cadence and history bounds (constants-first, ADR-0011).
const (
	// defaultSyncIntervalSeconds documents the periodic re-sync cadence's default — the
	// connector_connection.sync_interval_seconds column default (15 min). The live cadence
	// is now user-configurable per connection (on/off + interval): the idle wait reads it
	// via the GetSyncSchedule activity on every entry and arms the timer accordingly, so a
	// change applies live and survives continueAsNew (design-doc 0012: due / Trigger). A
	// TriggerSync still fires a run sooner; auto-sync off arms no timer at all.
	defaultSyncIntervalSeconds = 900

	// continueAsNew bounds — whichever trips first caps the event history so a
	// long-lived connection never blows the Temporal 50k-event limit (ADR-0034,
	// design-doc 0012: "~1,000 units processed OR ~15 minutes runtime"). We bound on
	// processed-units and runtime; the sync_unit table is the resumption state, so
	// the carry-over is just the input plus MidRun.
	continueAsNewUnits   = 1000
	continueAsNewRuntime = 15 * time.Minute
)

// ConnectionWorkflowInput is the tiny, payload-free seed for a ConnectionWorkflow.
// No cursors, no secrets — those live in connector state (Postgres) and storage.
type ConnectionWorkflowInput struct {
	ConnectionID string
	TenantID     string
	ConnectorID  string

	// MidRun is set only by continueAsNew: it tells the resumed workflow it is in the
	// MIDDLE of a drain, so it must NOT re-run StartRun (which would reset in-progress
	// units) and must skip straight back into the drain loop (design-doc 0012).
	MidRun bool
}

// ConnectionWorkflow is the per-connection lifecycle driver — the durable control
// plane over the sync_unit table. It holds only tiny locals ({phase, processed
// counter, rerunRequested}); it never holds the unit list, which is re-derived from
// sync_unit each batch (design-doc 0012, ADR-0034). The state machine is:
//
//	SCHEDULED → (TriggerSync | due timer) → RUNNING (StartRun? → DiscoverUnits →
//	drain sync_unit in bounded batches) → SETTLE → continueAsNew (re-arm).
//
// StopSync cancels the drain's child context cooperatively (ADR-0040): in-flight
// SyncUnit activities stop at a chunk boundary and return partial progress; a stop
// is settled to a resting state, not a failure. Disconnect ends the workflow.
func ConnectionWorkflow(ctx workflow.Context, input ConnectionWorkflowInput) error {
	log := workflow.GetLogger(ctx)

	// Signal channels. Buffered by Temporal; we drain them in the selectors below.
	triggerCh := workflow.GetSignalChannel(ctx, SignalTriggerSync)
	stopCh := workflow.GetSignalChannel(ctx, SignalStopSync)
	disconnectCh := workflow.GetSignalChannel(ctx, SignalDisconnect)
	selectionCh := workflow.GetSignalChannel(ctx, SignalUpdateSelection)
	refreshCh := workflow.GetSignalChannel(ctx, SignalRefreshCatalog)
	scheduleCh := workflow.GetSignalChannel(ctx, SignalUpdateSchedule)

	// A continueAsNew resume jumps straight back into the drain (MidRun): the prior
	// generation already discovered + started the run; re-running those would reset
	// progress. A fresh start waits for a trigger or the periodic timer first.
	if input.MidRun {
		_, err := runConnection(ctx, input, stopCh, triggerCh, disconnectCh, selectionCh, refreshCh, scheduleCh)
		return err
	}

	for {
		// Wait to run: a TriggerSync, the periodic due timer, a Disconnect, or a
		// selection change (which just updates which units are selected — no run).
		// Connecting does not auto-sync; the first run comes from a TriggerSync the
		// server sends on "Sync now" (design-doc 0009/0012).
		var run, disconnect, refresh bool

		// Read the user-configurable auto-sync cadence (on/off + interval) BEFORE building
		// the selector. Reading on every idle entry means a change applies live AND survives
		// continueAsNew (the resumed generation re-reads it here). On error, default to the
		// platform cadence so a transient DB blip doesn't strand the connection without a timer.
		var schedule activity.ScheduleResult
		if err := workflow.ExecuteActivity(controlCtx(ctx), actGetSyncSchedule, activity.ScheduleInput{
			ConnectionID: input.ConnectionID,
		}).Get(ctx, &schedule); err != nil {
			log.Info("connection_get_schedule_failed", "connection_id", input.ConnectionID, "error", err.Error())
			schedule = activity.ScheduleResult{Enabled: true, IntervalSeconds: defaultSyncIntervalSeconds}
		}

		sel := workflow.NewSelector(ctx)
		sel.AddReceive(triggerCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, nil)
			run = true
		})
		// Arm the periodic timer ONLY when auto-sync is enabled. When off, no timer is
		// added — the workflow then runs only on a manual TriggerSync (or other signals).
		if schedule.Enabled {
			interval := time.Duration(schedule.IntervalSeconds) * time.Second
			timer := workflow.NewTimer(ctx, interval)
			sel.AddFuture(timer, func(workflow.Future) { run = true })
		}
		sel.AddReceive(disconnectCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, nil)
			disconnect = true
		})
		sel.AddReceive(selectionCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, nil) // selection lives in sync_unit.selected; next run honors it
		})
		// RefreshCatalog while idle: run ONLY discovery (no drain) so the project
		// catalog is populated/refreshed for selection before the first sync.
		sel.AddReceive(refreshCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, nil)
			refresh = true
		})
		// An UpdateSchedule just wakes the select so the loop re-reads the cadence above
		// and re-arms the timer — a re-evaluate, not a run (no run flag set).
		sel.AddReceive(scheduleCh, func(c workflow.ReceiveChannel, _ bool) { c.Receive(ctx, nil) })
		// A StopSync with nothing running is a no-op — drain it so it doesn't wedge.
		sel.AddReceive(stopCh, func(c workflow.ReceiveChannel, _ bool) { c.Receive(ctx, nil) })
		sel.Select(ctx)

		if disconnect {
			// The server soft-deletes the row; the workflow just ends.
			log.Info("connection_workflow_disconnect", "connection_id", input.ConnectionID)
			return nil
		}
		if refresh {
			// Discover-only: refresh the catalog so the user can pick projects before
			// any sync. Cheap and idempotent — no StartRun, no drain. Errors are
			// non-fatal to the lifecycle (a failed discovery just leaves the prior
			// catalog; the next refresh/run retries).
			if err := workflow.ExecuteActivity(controlCtx(ctx), actDiscoverUnits, activity.DiscoverInput{
				ConnectionID: input.ConnectionID,
				Generation:   workflow.Now(ctx).UnixNano(),
			}).Get(ctx, nil); err != nil {
				log.Info("connection_refresh_catalog_failed", "connection_id", input.ConnectionID, "error", err.Error())
			}
			continue
		}
		if !run {
			continue
		}
		disconnected, err := runConnection(ctx, input, stopCh, triggerCh, disconnectCh, selectionCh, refreshCh, scheduleCh)
		if err != nil {
			return err // includes the continueAsNew "error" (Temporal's resume signal)
		}
		if disconnected {
			log.Info("connection_workflow_disconnect", "connection_id", input.ConnectionID)
			return nil
		}
	}
}

// runConnection drives one sync run end-to-end: StartRun (fresh only) → DiscoverUnits
// → the continuous worker-pool drain → SettleConnection. It returns a ContinueAsNewError when a
// history bound trips (the workflow resumes mid-drain), or nil when the run settles to
// rest (the caller loops back to wait for the next trigger/timer).
// runConnection returns (disconnected, err): disconnected is true when a Disconnect
// signal landed mid-run, so the caller ends the workflow.
func runConnection(
	ctx workflow.Context,
	input ConnectionWorkflowInput,
	stopCh, triggerCh, disconnectCh, selectionCh, refreshCh, scheduleCh workflow.ReceiveChannel,
) (bool, error) {
	log := workflow.GetLogger(ctx)
	start := workflow.Now(ctx)

	// rerunRequested coalesces a TriggerSync that lands mid-run: we don't start a
	// second concurrent drain (that would double-sync the connection); instead we set
	// the flag, finish the current drain, then run once more. A StopSync mid-run
	// cancels cooperatively and does NOT request a rerun (a stop is a stop).
	var (
		processed           int
		rerunRequested      bool
		stopRequested       bool
		disconnectRequested bool
	)

	// Drain under a cancellable child context so StopSync can cancel just the drain
	// (not the whole workflow). Cancelling it delivers cancellation to the in-flight
	// SyncUnit activities, which stop at a chunk boundary and return partial (ADR-0040).
	drainCtx, cancelDrain := workflow.WithCancel(ctx)

	// drainSignals reads the run-control signals NON-blockingly — called at each batch
	// boundary so cooperative cancellation lands at a chunk boundary (the only point a
	// drain can stop cleanly; ADR-0040). It returns whether the drain should stop now.
	// No background goroutine: draining inline keeps the run deterministic and avoids a
	// watcher that outlives the run. Signals are durable in Temporal, so one that lands
	// between polls is still there at the next boundary.
	drainSignals := func() (stop bool) {
		for stopCh.ReceiveAsync(nil) { // StopSync: cancel the drain cooperatively
			stopRequested = true
		}
		for disconnectCh.ReceiveAsync(nil) { // Disconnect: stop now; the wait loop ends the workflow
			stopRequested = true
			disconnectRequested = true
		}
		for triggerCh.ReceiveAsync(nil) { // TriggerSync mid-run: coalesce into one rerun
			rerunRequested = true
		}
		for selectionCh.ReceiveAsync(nil) { //nolint:revive // selection honored next run via sync_unit.selected
		}
		for refreshCh.ReceiveAsync(nil) { //nolint:revive // a run already discovers; drain so a mid-run refresh doesn't wedge
		}
		for scheduleCh.ReceiveAsync(nil) { //nolint:revive // a schedule change applies at the next idle entry (re-read); drain so it doesn't wedge
		}
		if stopRequested {
			cancelDrain()
		}
		return stopRequested
	}

	// StartRun resets prior-cycle terminal units for a fresh run only — a resume must
	// not reset mid-drain progress (design-doc 0012).
	if !input.MidRun {
		if err := workflow.ExecuteActivity(controlCtx(ctx), actStartRun, activity.RunInput{
			ConnectionID: input.ConnectionID,
		}).Get(ctx, nil); err != nil {
			return false, err
		}
		var discovered activity.DiscoverResult
		if err := workflow.ExecuteActivity(controlCtx(ctx), actDiscoverUnits, activity.DiscoverInput{
			ConnectionID: input.ConnectionID,
			// Generation: a monotonic stamp for mark-and-sweep (ADR-0037). Phase 4 owns
			// the durable counter; for now the run start time is a monotonic-enough seed.
			Generation: start.UnixNano(),
		}).Get(ctx, &discovered); err != nil {
			return false, err
		}
		log.Info("connection_run_discovered", "connection_id", input.ConnectionID, "units", discovered.Count)
	}

	// The drain loop is a CONTINUOUS worker-pool: it keeps up to P SyncUnit activities
	// in flight at all times, refilling a freed slot the instant any unit completes —
	// it does NOT batch-barrier (await a whole slice before launching the next). A
	// batch barrier collapses to concurrency 1 under skew (one 958-issue repo holds the
	// slice while its peers sit idle); the pool keeps the other P-1 slots busy on fresh
	// units (design-doc 0012, ADR-0036).
	//
	// inFlight is the set of unit ids currently running (|inFlight| ≤ P). NextUnitBatch
	// is filtered against it: a still-'syncing' unit that IS in flight must NOT be
	// double-launched, but a still-'syncing' unit that is NOT in flight (a chunk-budget
	// unit, Done=false, that just left the pool) MUST be re-launched — that is the
	// chunked-resume path (design-doc 0012). The selector waits for ANY in-flight unit;
	// its callback drops the unit from inFlight and counts it, freeing a slot to refill.
	inFlight := make(map[string]bool)
	selector := workflow.NewSelector(ctx)

	// stoppingCAN goes true once a history bound trips: we stop LAUNCHING new units but
	// keep draining the in-flight set, then continueAsNew (MidRun) once it empties so no
	// in-flight future is abandoned across the resume.
	stoppingCAN := false

	for {
		// capacityP reads the adaptive limit P from the admission controller (a cheap
		// control activity, never I/O in deterministic workflow code; ADR-0036). Re-read
		// on every refill so the pool tracks the limiter's learned ceiling as it adapts.
		// The micro level (SyncUnit's per-unit Acquire) is the hard backstop if P is stale.
		capacityP := func() (int, error) {
			var capacity activity.CapacityResult
			if err := workflow.ExecuteActivity(controlCtx(ctx), actCapacity, activity.CapacityInput{
				ConnectionID: input.ConnectionID,
				TenantID:     input.TenantID,
				ConnectorID:  input.ConnectorID,
			}).Get(ctx, &capacity); err != nil {
				return 0, err
			}
			if capacity.P < 1 {
				return 1, nil
			}
			return capacity.P, nil
		}

		// Refill: top the pool back up to P. Skip while stopping (cooperative stop or a
		// tripped continueAsNew bound) — we only drain what's already in flight then.
		launchedThisRefill := 0
		if !stopRequested && !stoppingCAN {
			p, err := capacityP()
			if err != nil {
				return false, err
			}
			for len(inFlight) < p {
				var batch activity.BatchResult
				if err := workflow.ExecuteActivity(controlCtx(ctx), actNextUnitBatch, activity.BatchInput{
					ConnectionID: input.ConnectionID,
					Limit:        p,
				}).Get(ctx, &batch); err != nil {
					return false, err
				}
				launched := 0
				for _, id := range batch.UnitIDs {
					if len(inFlight) >= p {
						break // pool full — leave the rest for the next refill
					}
					if inFlight[id] {
						continue // already running — must NOT double-launch
					}
					future := workflow.ExecuteActivity(syncCtx(drainCtx, input.ConnectorID), actSyncUnit, activity.SyncInput{
						ConnectionID: input.ConnectionID,
						TenantID:     input.TenantID,
						ConnectorID:  input.ConnectorID,
						UnitID:       id,
					})
					inFlight[id] = true
					unitID := id
					selector.AddFuture(future, func(f workflow.Future) {
						// One unit's error must not fail the run (it backs the unit off and
						// settles to error); a Stop cancellation surfaces here too — settle,
						// not fail. Free the slot so the next refill can use it.
						var res activity.SyncResult
						if err := f.Get(ctx, &res); err != nil {
							log.Info("sync_unit_failed", "connection_id", input.ConnectionID, "error", err.Error())
						}
						delete(inFlight, unitID)
						processed++
					})
					launched++
					launchedThisRefill++
				}
				// Nothing new to launch (every returned id is already in flight, or the
				// table is empty): stop refilling — re-querying would just spin.
				if launched == 0 {
					break
				}
			}
		}

		// Drained: pool empty and the last refill launched nothing → all units are
		// terminal (synced/error). Settle.
		if len(inFlight) == 0 && launchedThisRefill == 0 {
			break
		}

		// Wait for ONE in-flight unit to finish (its callback frees a slot). The drain
		// can also unblock via a cancelled drainCtx (Stop), which completes the in-flight
		// futures — handled by the same callbacks.
		selector.Select(ctx)

		// Read run-control signals at this completion boundary (the chunk boundary where
		// a drain can stop cleanly; ADR-0040). A Stop/Disconnect flips stopRequested and
		// cancels drainCtx, so we stop launching and just drain the in-flight set.
		drainSignals()

		// Bound history: once we've processed enough units or run long enough, STOP
		// launching new units and drain the in-flight set, then continueAsNew (MidRun) so
		// the resumed workflow re-enters the drain without re-running StartRun/Discover.
		// We don't abandon in-flight futures across the resume — sync_unit is the
		// resumption state, but a carried-but-unfinished activity would be lost work.
		if !stoppingCAN && shouldContinueAsNew(ctx, processed, start) {
			stoppingCAN = true
		}
		if stoppingCAN && len(inFlight) == 0 {
			cancelDrain()
			next := input
			next.MidRun = true
			return false, workflow.NewContinueAsNewError(ctx, ConnectionWorkflow, next)
		}
	}

	// Free the drain context before settling (a fresh run arms a new one next time).
	cancelDrain()

	// Settle: read the unit-status counts and set the connection's resting status. This
	// runs on the workflow ctx (not the cancelled drainCtx), so a stopped run still
	// settles cleanly — a stop is a resting state, not a failure (ADR-0040).
	var settled activity.SettleResult
	if err := workflow.ExecuteActivity(controlCtx(ctx), actSettleConnection, activity.SettleInput{
		ConnectionID: input.ConnectionID,
	}).Get(ctx, &settled); err != nil {
		return false, err
	}
	log.Info("connection_run_settled",
		"connection_id", input.ConnectionID,
		"status", settled.Status,
		"synced", settled.Synced,
		"errored", settled.Errored,
		"stopped", stopRequested,
	)

	// Disconnect mid-run: settled to rest, now end the workflow (the server soft-deletes
	// the row).
	if disconnectRequested {
		return true, nil
	}

	// Coalesced rerun: a TriggerSync arrived mid-run while we were draining. Run once
	// more (a fresh run — re-arm by continueAsNew so the new generation is clean). A
	// stop does not rerun.
	if rerunRequested && !stopRequested {
		next := input
		next.MidRun = false
		return false, workflow.NewContinueAsNewError(ctx, ConnectionWorkflow, next)
	}

	return false, nil
}

// shouldContinueAsNew reports whether a history bound has tripped. We bound on
// processed-units OR runtime (design-doc 0012). An event-count bound
// (GetCurrentHistoryLength ≈ 4000) is the alternative; processed/runtime is chosen
// because it reads naturally against the run model and the unit count is the real
// history driver here (one activity per unit launched by the pool).
func shouldContinueAsNew(ctx workflow.Context, processed int, start time.Time) bool {
	if processed >= continueAsNewUnits {
		return true
	}
	return workflow.Now(ctx).Sub(start) >= continueAsNewRuntime
}

// --- activity options --------------------------------------------------------

// controlCtx options the short control-plane activities (StartRun, DiscoverUnits,
// NextUnitBatch, SettleConnection): a 2-minute ceiling and a bounded retry with
// backoff. Auth errors are non-retryable (the activity returns them as such).
func controlCtx(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
			NonRetryableErrorTypes: []string{
				activityErrTypeAuth,
			},
		},
	})
}

// syncCtx options a SyncUnit drain: a 10-minute ceiling (a unit chunks across
// batches, so one call is bounded) and a 1-minute heartbeat timeout — a dead worker's
// in-flight unit is requeued once it stops heartbeating (ADR-0040). The drain context
// is the cancellable child, so a StopSync cancels these activities cooperatively.
//
// TaskQueue routes SyncUnit to the PER-CONNECTOR queue (sync-<connector>), not the
// shared ConnectionQueue: a throttled GitHub then can't starve Drive's activity slots
// (ADR-0034). Only the source-I/O activity is per-connector; the connector-agnostic
// control activities and the workflow itself stay on ConnectionQueue.
func syncCtx(ctx workflow.Context, connectorID string) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           igtemporal.SyncTaskQueue(connectorID),
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    time.Minute,
		// WaitForCancellation lets the in-flight SyncUnit return its partial result on a
		// Stop, rather than the workflow abandoning it — the cursor is persisted per item.
		WaitForCancellation: true,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
			NonRetryableErrorTypes: []string{
				activityErrTypeAuth,
			},
		},
	})
}

// activityErrTypeAuth mirrors the activity package's errTypeAuth label so the retry
// policy marks credential-rejection errors non-retryable (the activity already
// returns them via NewNonRetryableApplicationError; this keeps a custom retry policy
// from re-arming them).
const activityErrTypeAuth = "auth"
