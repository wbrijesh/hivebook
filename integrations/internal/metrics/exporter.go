package metrics

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"go.temporal.io/sdk/client"

	"integrations/internal/temporal"
)

// DB is the read-only slice of the data layer the exporter needs: the raw pool for the
// aggregate SLI queries (database.Store.SQLDB() satisfies this). Aggregate-only — the
// exporter never selects per-connection rows (design-doc 0012: connection_id is never a
// metric label).
type DB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Connectors is the set of connector IDs whose per-connector sync task queue the
// exporter polls for Temporal backlog (the KEDA scale signal). cmd passes the registry's
// ids; kept as a tiny interface so this package doesn't import connectors.
type Connectors []string

// RunExporter is the always-on backlog exporter (runs in cmd/integrations). Every
// `every` it refreshes the aggregate outcome SLIs from Postgres and the Temporal
// task-queue backlog, setting the gauges other code never touches inline. It blocks
// until ctx is done; run it in a goroutine. The gauges register on the default
// Prometheus registry, which the server's /metrics already serves.
//
// Why an exporter and not inline emission: freshness, backlog, and connection counts are
// fleet aggregates (max/count grouped by connector/region), not per-event increments —
// computing them is one cheap grouped query, not 50,000 series (design-doc 0012,
// "Observability"). The Temporal backlog is the load-bearing one: it is server-side, so
// it is non-zero even with zero workers, which is exactly what lets KEDA scale up from
// zero without the old River deadlock.
func RunExporter(ctx context.Context, db DB, tc client.Client, connectors Connectors, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	collect(ctx, db, tc, connectors) // once at start so /metrics is warm immediately
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			collect(ctx, db, tc, connectors)
		}
	}
}

func collect(ctx context.Context, db DB, tc client.Client, connectors Connectors) {
	collectPostgresSLIs(ctx, db)
	collectTemporalBacklog(ctx, tc, connectors)
}

// --- Postgres SLIs -----------------------------------------------------------

const (
	// Freshness: max lag (now − last_success_at) per connector/region across live
	// connections that have ever succeeded. A connection that never succeeded has a
	// NULL last_success_at and is excluded (it shows as backlog/needs_reauth instead,
	// not as infinite freshness lag).
	freshnessSQL = `
SELECT connector_id, region,
       EXTRACT(EPOCH FROM (now() - max(last_success_at)))
FROM connector_connection
WHERE deleted_at IS NULL AND last_success_at IS NOT NULL
GROUP BY connector_id, region`

	// Backlog: count of selected non-terminal units per connector — mirrors the
	// NextUnitBatch selection predicate (the work actually owed).
	backlogSQL = `
SELECT c.connector_id, count(*)
FROM sync_unit u
JOIN connector_connection c ON c.id = u.connection_id
WHERE c.deleted_at IS NULL
  AND u.selected = true
  AND u.status NOT IN ('synced', 'error', 'removed')
GROUP BY c.connector_id`

	// Connection counts by connector and resting status.
	connectionsSQL = `
SELECT connector_id, status, count(*)
FROM connector_connection
WHERE deleted_at IS NULL
GROUP BY connector_id, status`

	// Average current adaptive limit L per connector (ADR-0036), credential scope only.
	admissionSQL = `
SELECT connector_id, avg(concurrency_limit)
FROM admission_bucket
WHERE scope_type = 'credential' AND connector_id <> ''
GROUP BY connector_id`

	// Purge SLA breaches: artifacts owed deletion past their GDPR deadline (ADR-0039).
	purgeBreachSQL = `
SELECT count(*) FROM raw_artifact
WHERE purge_pending = true AND purge_deadline IS NOT NULL AND purge_deadline < now()`
)

func collectPostgresSLIs(ctx context.Context, db DB) {
	// Reset the vec gauges so a connector/region that drops to zero doesn't keep a stale
	// non-zero series — the grouped queries only return present groups.
	SyncFreshnessSeconds.Reset()
	SyncBacklogUnits.Reset()
	SyncConnections.Reset()
	AdmissionLimit.Reset()

	queryRows(ctx, db, freshnessSQL, func(rows *sql.Rows) error {
		var connector, region string
		var lag float64
		if err := rows.Scan(&connector, &region, &lag); err != nil {
			return err
		}
		SyncFreshnessSeconds.WithLabelValues(connector, region).Set(lag)
		return nil
	})

	queryRows(ctx, db, backlogSQL, func(rows *sql.Rows) error {
		var connector string
		var n float64
		if err := rows.Scan(&connector, &n); err != nil {
			return err
		}
		SyncBacklogUnits.WithLabelValues(connector).Set(n)
		return nil
	})

	queryRows(ctx, db, connectionsSQL, func(rows *sql.Rows) error {
		var connector, status string
		var n float64
		if err := rows.Scan(&connector, &status, &n); err != nil {
			return err
		}
		SyncConnections.WithLabelValues(connector, status).Set(n)
		return nil
	})

	queryRows(ctx, db, admissionSQL, func(rows *sql.Rows) error {
		var connector string
		var l float64
		if err := rows.Scan(&connector, &l); err != nil {
			return err
		}
		AdmissionLimit.WithLabelValues(connector).Set(l)
		return nil
	})

	var breaches float64
	if err := db.QueryRowContext(ctx, purgeBreachSQL).Scan(&breaches); err != nil {
		slog.Warn("metrics_exporter_purge_breach", "error", err.Error())
	} else {
		PurgeSLABreaches.Set(breaches)
	}
}

// --- Temporal task-queue backlog --------------------------------------------

// collectTemporalBacklog sets hivebook_temporal_backlog{queue} from Temporal's
// server-side backlog: the ConnectionQueue plus each sync-<connector> queue. We use
// DescribeTaskQueueEnhanced with ReportStats so we read ApproximateBacklogCount — the
// real count of pending workflow+activity tasks the server holds. It is non-zero even
// when no worker exists, which is the property KEDA scales on (a triggered sync wakes a
// worker from zero; the old River scaler counted only states the parked work never
// reached, hence the deadlock — design-doc 0012, ADR-0034).
//
// If the (dev-server) build does not support the enhanced describe / stats, we fall back
// to the basic DescribeTaskQueue poller count and log the limitation: poller count is a
// liveness proxy, NOT a backlog, so it cannot scale from zero — documented so the infra
// agent points KEDA at a server that supports stats.
func collectTemporalBacklog(ctx context.Context, tc client.Client, connectors Connectors) {
	queues := make([]string, 0, len(connectors)+1)
	queues = append(queues, temporal.ConnectionQueue)
	for _, c := range connectors {
		queues = append(queues, temporal.SyncTaskQueue(c))
	}
	for _, q := range queues {
		TemporalBacklog.WithLabelValues(q).Set(taskQueueBacklog(ctx, tc, q))
	}
}

// taskQueueBacklog returns the approximate backlog for one queue, summing the workflow
// and activity task-type stats. On any error it returns 0 (a missing reading must not
// pin the scaler high) after logging.
func taskQueueBacklog(ctx context.Context, tc client.Client, queue string) float64 {
	desc, err := tc.DescribeTaskQueueEnhanced(ctx, client.DescribeTaskQueueEnhancedOptions{
		TaskQueue: queue,
		TaskQueueTypes: []client.TaskQueueType{
			client.TaskQueueTypeWorkflow,
			client.TaskQueueTypeActivity,
		},
		ReportStats: true,
	})
	if err != nil {
		slog.Warn("metrics_exporter_describe_task_queue", "queue", queue, "error", err.Error())
		return 0
	}

	var backlog float64
	for _, version := range desc.VersionsInfo {
		for _, typeInfo := range version.TypesInfo {
			if typeInfo.Stats != nil {
				backlog += float64(typeInfo.Stats.ApproximateBacklogCount)
			}
		}
	}
	return backlog
}

// --- query helpers -----------------------------------------------------------

func queryRows(ctx context.Context, db DB, query string, scan func(*sql.Rows) error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		slog.Warn("metrics_exporter_query", "error", err.Error())
		return
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			slog.Warn("metrics_exporter_scan", "error", err.Error())
			return
		}
	}
	if err := rows.Err(); err != nil {
		slog.Warn("metrics_exporter_rows", "error", err.Error())
	}
}
