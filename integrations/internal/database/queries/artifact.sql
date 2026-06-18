-- Raw artifact (manifest) queries (design-doc 0008/0009). Raw bytes live in
-- object storage; this indexes them.

-- name: GetArtifactContentHash :one
SELECT content_hash FROM raw_artifact
WHERE connection_id = @connection_id AND external_id = @external_id;

-- name: ListArtifacts :many
-- The tenant's synced items, newest first — the frontend's file list. Optionally
-- scoped to one connection (the source→files deep link): pass '' for all. Both
-- sides compared as text so the empty sentinel never casts to uuid. Soft-deleted
-- (pending-purge) artifacts are hidden. The committed-unit filter hides items whose
-- sync_unit has never completed a full pass (an in-progress or stopped initial
-- backfill), so partial data never surfaces (ADR-0033). The join is on the unit's
-- external_id = the artifact's sync_container_id (the unit, not the manifest
-- container; they differ for Drive: drive vs folder).
SELECT id, connection_id, source, source_native_kind, container_name, external_id, source_url, ingested_at
FROM raw_artifact ra
WHERE ra.tenant_id = @tenant_id AND ra.deleted_at IS NULL
  AND (@connection_id::text = '' OR ra.connection_id::text = @connection_id::text)
  AND EXISTS (
    SELECT 1 FROM sync_unit u
    WHERE u.connection_id = ra.connection_id
      AND u.external_id = ra.sync_container_id
      AND u.committed_at IS NOT NULL
  )
ORDER BY ra.ingested_at DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountArtifacts :one
SELECT count(*) FROM raw_artifact ra
WHERE ra.tenant_id = @tenant_id AND ra.deleted_at IS NULL
  AND (@connection_id::text = '' OR ra.connection_id::text = @connection_id::text)
  AND EXISTS (
    SELECT 1 FROM sync_unit u
    WHERE u.connection_id = ra.connection_id
      AND u.external_id = ra.sync_container_id
      AND u.committed_at IS NOT NULL
  );

-- name: GetArtifactForContent :one
-- Resolve an artifact's object key + label for serving its content (tenant-scoped).
SELECT object_key, external_id, source_native_kind FROM raw_artifact
WHERE id = @id AND tenant_id = @tenant_id AND deleted_at IS NULL;

-- name: UpsertRawArtifact :exec
-- sync_container_id is the SYNC unit's external_id (the unit the item was pulled
-- from) — distinct from container_id, the per-artifact manifest container used for
-- filing priors (ADR-0035; for GitHub they coincide, for Google Docs the sync unit
-- is the drive while container_id is the folder). The visibility gate joins sync_unit
-- on sync_container_id (ADR-0033).
INSERT INTO raw_artifact (
    tenant_id, region, connection_id, source, source_native_kind,
    container_id, sync_container_id, container_kind, container_name, external_id, source_url,
    object_key, content_hash, source_created_at, source_updated_at, ingested_at, updated_at
) VALUES (
    @tenant_id, @region, @connection_id, @source, @source_native_kind,
    @container_id, @sync_container_id, @container_kind, @container_name, @external_id, @source_url,
    @object_key, @content_hash, @source_created_at, @source_updated_at, now(), now()
)
ON CONFLICT (connection_id, external_id) DO UPDATE SET
    object_key        = EXCLUDED.object_key,
    content_hash      = EXCLUDED.content_hash,
    source_url        = EXCLUDED.source_url,
    container_name    = EXCLUDED.container_name,
    sync_container_id = EXCLUDED.sync_container_id,
    source_updated_at = EXCLUDED.source_updated_at,
    -- ingested_at is immutable (true first-ingest time, the Files "newest first"
    -- key); a re-sync of a changed item must not jump it to the top. updated_at
    -- tracks the last change instead (design-doc 0011 P1).
    updated_at        = now();

-- --- erasure deletion outbox (ADR-0039) --------------------------------------

-- name: MarkArtifactsPurgePending :exec
-- The manifest row is NOT dropped on delete; it is marked purge_pending with an SLA
-- deadline (typically a 30-day GDPR window). The object delete is attempted, then —
-- and only then — the row is removed (ConfirmArtifactDeletes). Anything else stays
-- purge_pending for GC retry; a row past purge_deadline pages.
UPDATE raw_artifact
SET purge_pending = true, purge_deadline = @purge_deadline, deleted_at = now(), updated_at = now()
WHERE id = ANY(@ids::uuid[]);

-- name: ListPurgePendingArtifacts :many
-- The outbox: rows whose object delete is still owed, oldest deadline first so the
-- nearest-to-SLA work runs first. The worker deletes each object then confirms.
SELECT id, object_key, purge_deadline FROM raw_artifact
WHERE purge_pending
ORDER BY purge_deadline NULLS LAST
LIMIT @row_limit;

-- name: ConfirmArtifactDeletes :execrows
-- After the object delete confirms (204 or 404-already-gone), drop the manifest rows.
DELETE FROM raw_artifact WHERE id = ANY(@ids::uuid[]) AND purge_pending;

-- name: ListPurgePendingPastDeadline :many
-- Rows that have NOT confirmed deletion within the SLA — these page (ADR-0039); they
-- do not sit quietly in the retry loop. The SLA is measured from when the purge was
-- REQUESTED (deleted_at, the moment the row entered the outbox), not from
-- purge_deadline (which is the grace-before-bytes-deleted window, a shorter clock).
-- @sla_cutoff is now() − the GDPR SLA; a row still purge_pending with deleted_at
-- before it has blown the contractual window and must be paged.
SELECT id, object_key, deleted_at FROM raw_artifact
WHERE purge_pending AND deleted_at IS NOT NULL AND deleted_at < @sla_cutoff
ORDER BY deleted_at
LIMIT @row_limit;

-- name: MarkConnectionArtifactsPurgePending :exec
-- Immediate erasure on disconnect (ADR-0039): mark ALL a connection's live artifacts
-- purge_pending with purge_deadline = now() (no grace — the user revoked the
-- connection). Tenant-scoped so a connection id alone can never enqueue another
-- tenant's bytes for deletion. The outbox then guarantees-or-pages the actual delete.
UPDATE raw_artifact
SET purge_pending = true, purge_deadline = now(), deleted_at = COALESCE(deleted_at, now()), updated_at = now()
WHERE connection_id = @connection_id AND tenant_id = @tenant_id AND NOT purge_pending;

-- name: MarkRemovedUnitArtifactsPurgePending :exec
-- The sweep's second step (ADR-0037): after units are tombstoned ('removed'), mark
-- their artifacts purge_pending with a grace deadline (now + grace) so the bytes are
-- not deleted until the grace window — the second line of defence behind the
-- don't-sweep-on-suspicious-discovery guard. deleted_at is stamped now so the item
-- leaves Files immediately and the SLA clock starts at the request.
UPDATE raw_artifact ra
SET purge_pending = true, purge_deadline = @purge_deadline, deleted_at = COALESCE(ra.deleted_at, now()), updated_at = now()
WHERE ra.connection_id = @connection_id AND NOT ra.purge_pending
  AND ra.sync_container_id IN (
    SELECT u.external_id FROM sync_unit u
    WHERE u.connection_id = @connection_id AND u.status = 'removed'
  );

-- --- legacy purge (soft-deleted artifacts past a cutoff) ---------------------

-- name: ListPurgeableArtifacts :many
-- Soft-deleted before the cutoff (the most recent IST midnight) — their object keys
-- are deleted from storage, then the rows are dropped by id.
SELECT id, object_key FROM raw_artifact
WHERE deleted_at IS NOT NULL AND deleted_at < @cutoff
ORDER BY id
LIMIT @row_limit;

-- name: DeleteArtifactsByIDs :execrows
DELETE FROM raw_artifact WHERE id = ANY(@ids::uuid[]);

-- name: DeletePurgeableConnections :execrows
-- After their artifacts are purged; CASCADE removes any leftover children.
DELETE FROM connector_connection
WHERE deleted_at IS NOT NULL AND deleted_at < @cutoff;
