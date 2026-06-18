-- Connection queries (design-doc 0009/0012). Tenant-scoped reads filter by
-- tenant_id; the worker loads by id across tenants (it runs server-side).

-- name: UpsertConnection :one
-- On reconnect, keep the existing refresh token when the new exchange omits one
-- (Google only returns it on first consent) — never clobber a good token (#3).
INSERT INTO connector_connection (
    tenant_id, region, connector_id, status, account,
    access_token, refresh_token, token_expiry, scopes, installation_id, external_id, updated_at
) VALUES (
    @tenant_id, @region, @connector_id, 'connected', @account,
    @access_token, @refresh_token, @token_expiry, @scopes, @installation_id, @external_id, now()
)
ON CONFLICT (tenant_id, connector_id, external_id) DO UPDATE SET
    status          = 'connected',
    account         = EXCLUDED.account,
    installation_id = EXCLUDED.installation_id,
    access_token    = EXCLUDED.access_token,
    refresh_token = CASE
        WHEN EXCLUDED.refresh_token IS NOT NULL AND length(EXCLUDED.refresh_token) > 0
        THEN EXCLUDED.refresh_token
        ELSE connector_connection.refresh_token END,
    token_expiry  = EXCLUDED.token_expiry,
    scopes        = EXCLUDED.scopes,
    last_error    = '',
    deleted_at    = NULL, -- reconnecting un-deletes a connection pending purge
    updated_at    = now()
RETURNING *;

-- name: GetConnection :one
SELECT * FROM connector_connection
WHERE id = @id AND tenant_id = @tenant_id AND deleted_at IS NULL;

-- name: GetConnectionByID :one
-- A soft-deleted connection reads as gone, so the worker drops its jobs.
SELECT * FROM connector_connection WHERE id = @id AND deleted_at IS NULL;

-- name: ListConnections :many
SELECT * FROM connector_connection
WHERE tenant_id = @tenant_id AND deleted_at IS NULL
ORDER BY connector_id;

-- name: SetNeedsReauth :exec
UPDATE connector_connection
SET status = 'needs_reauth', last_error = @last_error, updated_at = now()
WHERE id = @id;

-- name: SetConnectionStatus :exec
-- The Temporal SETTLE step folds unit statuses × phase into the connection status
-- (design-doc 0012): backfilling / connected / connected+warnings / error.
UPDATE connector_connection
SET status = @status, last_error = @last_error, updated_at = now()
WHERE id = @id;

-- name: RecordSyncAttempt :exec
-- Stamp the start of a run (the Temporal RUNNING phase begins). Clears last_error.
UPDATE connector_connection
SET last_attempt_at = now(), last_error = '', updated_at = now()
WHERE id = @id;

-- name: RecordSyncSuccess :exec
-- A clean drain settles the connection: stamp success and reset the failure streak.
UPDATE connector_connection
SET last_success_at = now(), consecutive_failures = 0,
    status = 'connected', last_error = '', updated_at = now()
WHERE id = @id;

-- name: RecordSyncFailure :exec
-- A failed run bumps the streak ONCE (per run, not per unit) and records the error.
UPDATE connector_connection
SET consecutive_failures = consecutive_failures + 1,
    status = 'error', last_error = @last_error, updated_at = now()
WHERE id = @id;

-- name: DeleteConnection :execrows
-- Soft delete: mark for purge and disable. The midnight purge hard-deletes the row
-- and its files; reconnecting before then un-deletes it (UpsertConnection clears
-- deleted_at). Tenant-scoped; returns the row count so the caller can reject a
-- cross-tenant id instead of silently affecting nothing.
UPDATE connector_connection
SET deleted_at = now(), status = 'disabled', updated_at = now()
WHERE id = @id AND tenant_id = @tenant_id;

-- name: SoftDeleteConnectionArtifacts :exec
-- Tenant-scoped: a connection id alone must never let one tenant mark another
-- tenant's artifacts for purge (the connection_id is not a secret — design-doc
-- 0011 P0). Pair with the tenant-scoped DeleteConnection above.
UPDATE raw_artifact SET deleted_at = now()
WHERE connection_id = @connection_id AND tenant_id = @tenant_id AND deleted_at IS NULL;

-- name: UndeleteConnectionArtifacts :exec
-- Tenant-scoped for the same reason as the soft-delete above.
UPDATE raw_artifact SET deleted_at = NULL
WHERE connection_id = @connection_id AND tenant_id = @tenant_id AND deleted_at IS NOT NULL;

-- name: UpdateTokens :exec
-- Persist a rotated access AND refresh token. Use only when the rotation actually
-- returned a refresh token; when it didn't, use UpdateAccessToken so a good stored
-- refresh token is never blanked (design-doc 0010/0011).
UPDATE connector_connection
SET access_token = @access_token, refresh_token = @refresh_token,
    token_expiry = @token_expiry, updated_at = now()
WHERE id = @id;

-- name: SetCatalogDiscovered :exec
-- Mark the connection's catalog as discovered (design-doc 0012). Called at the end of
-- every healthy, non-degraded DiscoverUnits pass — NULL only before the first one, so
-- the UI can tell "discovery hasn't finished yet" (cold-start) from "no projects".
UPDATE connector_connection
SET catalog_discovered_at = now(), updated_at = now()
WHERE id = @id;

-- name: MarkConnectionOnboarded :exec
-- Flip the onboarded flag the first time the user saves a project selection
-- (SetProjectSelection): the act of configuring the source completes onboarding. The
-- frontend routes a not-onboarded connection to its manage page, an onboarded one to
-- its details page.
UPDATE connector_connection
SET onboarded = true, updated_at = now()
WHERE id = @id;

-- name: UpdateSyncSchedule :exec
-- Set the connection's auto-sync cadence (on/off + interval). The ConnectionWorkflow
-- reads it back via GetSyncSchedule on every idle entry, so a change applies live
-- (design-doc 0012). Validated in the handler; this is the raw write.
UPDATE connector_connection
SET auto_sync_enabled = @auto_sync_enabled, sync_interval_seconds = @sync_interval_seconds, updated_at = now()
WHERE id = @id;

-- name: GetSyncSchedule :one
-- The workflow's read-on-idle of the auto-sync cadence: the GetSyncSchedule control
-- activity calls this each time it parks, so config survives continueAsNew.
SELECT auto_sync_enabled, sync_interval_seconds FROM connector_connection
WHERE id = @id;

-- name: UpdateAccessToken :exec
-- Persist a rotated access token while preserving the stored refresh token — the
-- reuse token source omits the refresh token when it hasn't changed, and blanking
-- it would break the next refresh (design-doc 0010/0011 P1).
UPDATE connector_connection
SET access_token = @access_token, token_expiry = @token_expiry, updated_at = now()
WHERE id = @id;
