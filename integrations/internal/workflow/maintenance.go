package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"integrations/internal/activity"
)

// MaintenanceWorkflow is the periodic erasure + GC driver (ADR-0039, ADR-0037). It is a
// long-lived singleton — ONE running instance for the whole namespace, fixed workflow id
// MaintenanceWorkflowID — not one-per-connection. Each iteration:
//
//  1. PurgeArtifacts, looped until the outbox drains (no pending past its deadline), so a
//     backlog is cleared in bounded batches across activity calls.
//  2. CheckPurgeSLA — surface any purge_pending past the GDPR window so it PAGES.
//  3. GCRemovedUnits — hard-delete tombstoned units whose grace has elapsed (their bytes
//     were erased in step 1).
//
// It sleeps maintenanceInterval, then continueAsNew so history stays bounded regardless of
// how long it runs.
//
// Start it once (a singleton; do NOT register one per connection). Example start call —
// WorkflowIDReusePolicy REJECT_DUPLICATE keeps a second start from racing a running one:
//
//	started by: client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
//	    ID:                       workflow.MaintenanceWorkflowID,
//	    TaskQueue:                temporal.ConnectionQueue,
//	    WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
//	}, workflow.MaintenanceWorkflow)
//
// (a `just maintenance-start` target or a one-shot admin command does this; the worker
// does NOT auto-start it, so the build/tests don't require Temporal running.)
func MaintenanceWorkflow(ctx workflow.Context) error {
	log := workflow.GetLogger(ctx)

	// 1. Drain the erasure outbox in bounded batches until empty (or a safety cap on
	//    iterations so one pass can't loop forever on a persistently-failing delete —
	//    those stay purge_pending and are caught by CheckPurgeSLA / the next pass).
	var totalDeleted, totalFailed int
	for i := 0; i < maintenanceMaxPurgeBatches; i++ {
		var purge activity.PurgeResult
		if err := workflow.ExecuteActivity(maintenanceCtx(ctx), actPurgeArtifacts, activity.PurgeInput{}).
			Get(ctx, &purge); err != nil {
			return err
		}
		totalDeleted += purge.Deleted
		totalFailed += purge.Failed
		// Stop when a pass cleared nothing new: either the outbox is empty or every
		// remaining row is failing (left for the next interval + the SLA alarm).
		if purge.Deleted == 0 && purge.Orphaned == 0 {
			break
		}
	}

	// 2. SLA check — a stuck delete past the GDPR window pages (the activity sets the
	//    breach gauge + an error log).
	var sla activity.SLAResult
	if err := workflow.ExecuteActivity(maintenanceCtx(ctx), actCheckPurgeSLA).Get(ctx, &sla); err != nil {
		return err
	}

	// 3. GC tombstoned units past their grace (their bytes were erased in step 1).
	var gc activity.GCResult
	if err := workflow.ExecuteActivity(maintenanceCtx(ctx), actGCRemovedUnits).Get(ctx, &gc); err != nil {
		return err
	}

	log.Info("maintenance_pass",
		"purged", totalDeleted, "purge_failed", totalFailed,
		"sla_breached", sla.Breached, "units_gced", gc.Deleted,
	)

	// Sleep until the next pass, then continueAsNew to bound history. A Disconnect-style
	// shutdown isn't needed: it's a singleton that runs for the life of the deployment.
	if err := workflow.Sleep(ctx, maintenanceInterval); err != nil {
		return err
	}
	return workflow.NewContinueAsNewError(ctx, MaintenanceWorkflow)
}

// MaintenanceWorkflowID is the fixed id that makes MaintenanceWorkflow a singleton.
const MaintenanceWorkflowID = "maintenance"

// Maintenance cadence and bounds (constants-first, ADR-0011).
const (
	// maintenanceInterval is how often the erasure + GC pass runs. Daily: the grace and
	// SLA windows are in days, so a finer cadence buys nothing; a continueAsNew per day
	// keeps history trivially bounded.
	maintenanceInterval = 24 * time.Hour

	// maintenanceMaxPurgeBatches caps the per-interval purge loop so a persistently
	// failing delete can't spin it forever — the remainder waits for the next interval
	// and is surfaced by CheckPurgeSLA. With purgeBatch=200, this drains up to 10k/pass.
	maintenanceMaxPurgeBatches = 50
)

// Activity names the MaintenanceWorkflow invokes — must match the activity method names.
const (
	actPurgeArtifacts = "PurgeArtifacts"
	actCheckPurgeSLA  = "CheckPurgeSLA"
	actGCRemovedUnits = "GCRemovedUnits"
)

// maintenanceCtx options the maintenance activities: a generous ceiling (a purge batch
// does up to purgeBatch object deletes) and a bounded retry. No NonRetryable auth type —
// these touch storage + Postgres, not a credentialed source.
func maintenanceCtx(ctx workflow.Context) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    5,
		},
	})
}
