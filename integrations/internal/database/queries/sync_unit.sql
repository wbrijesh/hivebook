-- Sync-unit queries (design-doc 0012). A sync_unit is the connector-defined unit of
-- sync (ADR-0035). Discovery (mark-and-sweep, ADR-0037) keeps the catalog fresh and
-- tombstones what's gone; the user picks what to sync; the worker advances the cursor
-- (ADR-0031/0038), stamps committed_at for visibility (ADR-0033), and backs off poison
-- units. This table replaces the old project + watermark tables.

-- name: UpsertSyncUnit :exec
-- Discovery upserts each unit it finds, stamping the current generation/last_seen_at
-- (the "mark" of mark-and-sweep). A new unit defaults to NOT selected (the column
-- default — this INSERT never names `selected`); the user opts in. Re-discovering an
-- existing one refreshes its label and mark but never touches the user's selection,
-- its cursor, or its sync status.
INSERT INTO sync_unit (connection_id, external_id, name, kind, generation, last_seen_at)
VALUES (@connection_id, @external_id, @name, @kind, @generation, now())
ON CONFLICT (connection_id, external_id) DO UPDATE SET
    name         = EXCLUDED.name,
    kind         = EXCLUDED.kind,
    generation   = EXCLUDED.generation,
    last_seen_at = now(),
    updated_at   = now();

-- name: GetSyncUnit :one
SELECT * FROM sync_unit
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: SetUnitTotal :exec
-- Store the best-effort per-unit total-item estimate discovery fetched up front
-- (connector.EstimateTotals) so the UI can show synced/total and sort by total. A
-- unit absent from the estimate keeps its prior value (this is never called for it),
-- so a partial estimate never zeroes a known total. Best-effort: a stale estimate
-- self-corrects on the next discovery. Stamps total_estimate_at = now() so the UI
-- knows WHEN the ceiling was last checked (null = never).
UPDATE sync_unit
SET total_estimate = @total_estimate, total_estimate_at = now(), updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: ListSyncUnits :many
-- Tenant-scoped via the parent connection. Stable order for the settings list.
-- @query is an optional case-insensitive name filter (the manage page's debounced
-- search); empty means no filter, so WatchProjects passes "" and gets the full list.
SELECT u.external_id, u.name, u.kind, u.selected, u.status,
       u.cursor, u.last_error, u.committed_at, u.artifact_count, u.total_estimate, u.total_estimate_at, u.last_seen_at
FROM sync_unit u
JOIN connector_connection c ON c.id = u.connection_id
WHERE u.connection_id = @connection_id AND c.tenant_id = @tenant_id
  AND (@query::text = '' OR u.name ILIKE '%' || @query || '%')
ORDER BY u.external_id;

-- name: ListSelectedUnitIDs :many
-- The unit external_ids a sync should drain.
SELECT external_id FROM sync_unit
WHERE connection_id = @connection_id AND selected = true;

-- name: SetUnitSelection :exec
-- Replace the selection in one shot: selected = (external_id ∈ @selected_ids).
UPDATE sync_unit
SET selected = (external_id = ANY(@selected_ids::text[])), updated_at = now()
WHERE connection_id = @connection_id;

-- name: SetUnitStatus :exec
UPDATE sync_unit
SET status = @status, updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: SetUnitCursor :exec
-- Advance the high-water-mark cursor (ADR-0031/0038). Written in the same Postgres
-- transaction as the manifest rows so the cursor and the artifacts move atomically.
UPDATE sync_unit
SET cursor = @cursor, updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: IncrementUnitArtifactCount :exec
-- Bump the live synced count as a NEW artifact lands (a dedup-skip does not call this).
-- Written in the SAME transaction as the manifest+cursor advance (ADR-0038) so progress
-- is atomic with the commit. This is a running estimate: CommitUnit later recomputes the
-- exact count from the manifest, so the live counter self-corrects (it can transiently
-- climb above the committed count during a delta re-sync — reconciled at commit).
UPDATE sync_unit
SET artifact_count = artifact_count + @delta, updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: CommitUnit :exec
-- Stamp committed_at on the first full pass (ADR-0033): the unit's artifacts become
-- visible. Recompute artifact_count from the manifest so it reflects dedup, not a
-- running tally. Subsequent passes refresh the count but keep the original commit.
UPDATE sync_unit
SET committed_at = COALESCE(committed_at, now()),
    status = 'synced',
    last_error = '',
    artifact_count = (
        SELECT count(*) FROM raw_artifact ra
        WHERE ra.connection_id = @connection_id
          AND ra.sync_container_id = @external_id
          AND ra.deleted_at IS NULL
    ),
    updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: ResetActiveUnits :exec
-- Normalize a connection's in-progress units back to idle when a sync is stopped
-- (design-doc 0012). Queued/syncing rows return to idle; synced rows are untouched,
-- so committed work and its committed_at gate stand. A later sync re-drains them.
UPDATE sync_unit
SET status = 'idle', updated_at = now()
WHERE connection_id = @connection_id AND status IN ('queued', 'syncing');

-- --- poison-unit backoff (ADR-0036) -----------------------------------------

-- name: SetUnitRetry :exec
-- A unit failed: record the error and arm an exponential backoff so one dead repo
-- doesn't burn a slot — and the rate budget — every pass.
UPDATE sync_unit
SET status = 'error', last_error = @last_error, next_retry_at = @next_retry_at, updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: ClearUnitRetry :exec
-- A previously-poisoned unit succeeded: clear the backoff and error.
UPDATE sync_unit
SET last_error = '', next_retry_at = NULL, updated_at = now()
WHERE connection_id = @connection_id AND external_id = @external_id;

-- --- mark-and-sweep discovery lifecycle (ADR-0037) --------------------------

-- name: CountLiveUnits :one
-- The connection's current count of live (non-removed) units — the "last known
-- good" the degraded-discovery guard compares the fresh discovery against. A
-- discovery far below this (or empty) is a degraded reconcile and must NOT sweep
-- (ADR-0037), so a transient blip can never tombstone the corpus.
SELECT count(*) FROM sync_unit
WHERE connection_id = @connection_id AND status <> 'removed';

-- name: MaxUnitGeneration :one
-- The highest generation stamped on this connection's units. Discovery picks the
-- next pass's generation as max+1 so the sweep ("generation < current") tombstones
-- exactly the units this pass did not re-observe. COALESCE so an empty connection
-- starts at 0 → first pass runs as generation 1.
SELECT COALESCE(max(generation), 0)::bigint FROM sync_unit
WHERE connection_id = @connection_id;

-- name: ListConnectionsWithRemovedUnits :many
-- Connections that have tombstoned units past the grace cutoff — the GC work list
-- for the maintenance driver. Distinct so a connection appears once regardless of
-- how many of its units are removed.
SELECT DISTINCT connection_id FROM sync_unit
WHERE status = 'removed' AND updated_at < @cutoff;

-- name: SweepUnseenUnits :execrows
-- The "sweep": soft-tombstone selected units NOT observed in the latest discovery
-- generation (status -> 'removed'). The caller only runs this after a SANITY-GATED
-- discovery (a non-empty, non-suspiciously-shrunk result), so a degraded discovery
-- never tombstones a tenant's corpus. Returns the count for observability.
UPDATE sync_unit
SET status = 'removed', updated_at = now()
WHERE connection_id = @connection_id
  AND generation < @generation
  AND status <> 'removed';

-- name: ListRemovedUnits :many
-- Removed (tombstoned) units past their grace period, for GC. The grace window is
-- the second line of defence behind the don't-sweep-on-suspicious-discovery rule.
SELECT external_id FROM sync_unit
WHERE connection_id = @connection_id
  AND status = 'removed' AND updated_at < @cutoff
ORDER BY external_id;

-- name: SoftDeleteRemovedUnitArtifacts :exec
-- Soft-delete the artifacts of tombstoned units so they leave Files now and purge at
-- the cutoff. Matches on sync_container_id (the unit's external_id).
UPDATE raw_artifact SET deleted_at = now()
WHERE raw_artifact.connection_id = @connection_id
  AND raw_artifact.deleted_at IS NULL
  AND raw_artifact.sync_container_id IN (
    SELECT u.external_id FROM sync_unit u
    WHERE u.connection_id = @connection_id AND u.status = 'removed'
  );

-- name: SoftDeleteDeselectedUnitArtifacts :exec
-- After a selection change: mark the files of now-deselected units for purge.
UPDATE raw_artifact SET deleted_at = now()
WHERE raw_artifact.connection_id = @connection_id AND raw_artifact.deleted_at IS NULL
  AND raw_artifact.sync_container_id IN (
    SELECT u.external_id FROM sync_unit u
    WHERE u.connection_id = @connection_id AND u.selected = false
  );

-- name: UndeleteSelectedUnitArtifacts :exec
-- ...and un-delete the files of (re)selected units pending purge.
UPDATE raw_artifact SET deleted_at = NULL
WHERE raw_artifact.connection_id = @connection_id AND raw_artifact.deleted_at IS NOT NULL
  AND raw_artifact.sync_container_id IN (
    SELECT u.external_id FROM sync_unit u
    WHERE u.connection_id = @connection_id AND u.selected = true
  );

-- name: GCRemovedUnits :execrows
-- Hard-delete tombstoned units past the grace cutoff (their artifacts are purged
-- separately by the erasure outbox). An empty connection prunes nothing.
DELETE FROM sync_unit
WHERE connection_id = @connection_id
  AND status = 'removed' AND updated_at < @cutoff;
