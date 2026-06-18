-- Sync-run history (design-doc 0012): per-connection / per-unit run lineage, the
-- on-demand detail behind the aggregate SLIs (connection_id is never a metric label).

-- name: InsertSyncRun :exec
INSERT INTO sync_run (
    tenant_id, connection_id, connector_id, unit_id, result, artifacts, error, started_at, finished_at
) VALUES (
    @tenant_id, @connection_id, @connector_id, @unit_id, @result, @artifacts, @error, @started_at, now()
);

-- name: ListSyncRuns :many
-- A connection's recent runs, newest first — the connector-health view.
SELECT id, unit_id, result, artifacts, error, started_at, finished_at
FROM sync_run
WHERE connection_id = @connection_id
ORDER BY started_at DESC
LIMIT @row_limit;

-- name: DeleteSyncRunsBefore :execrows
-- Retention: drop sync-run history older than the cutoff so the table stays bounded
-- (the purge job calls this daily — design-doc 0011 P3-6).
DELETE FROM sync_run WHERE finished_at < @cutoff;
