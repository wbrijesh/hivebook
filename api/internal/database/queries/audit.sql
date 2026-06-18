-- Audit-trail queries (ADR-0010). Consumed via the generated package; nothing
-- else writes SQL.

-- name: RecordAuditEvent :exec
INSERT INTO audit_events (tenant_id, actor_id, actor_email, action, target, metadata)
VALUES (@tenant_id, @actor_id, @actor_email, @action, @target, @metadata);

-- name: ListAuditEvents :many
-- A tenant's trail, newest first. Pagination via limit/offset.
SELECT id, tenant_id, actor_id, actor_email, action, target, metadata, created_at
FROM audit_events
WHERE tenant_id = @tenant_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountAuditEvents :one
SELECT count(*) FROM audit_events WHERE tenant_id = $1;
