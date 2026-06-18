-- Audit trail: one row per privileged action in a workspace (ADR-0010, audit is a
-- day-zero primitive). Queryable per tenant, newest-first; not exportable yet
-- (roadmap v0.1). actor_id is the ZITADEL user sub; metadata is reserved for
-- structured detail we don't surface yet.
CREATE TABLE audit_events (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    actor_id    text        NOT NULL,
    actor_email text,
    action      text        NOT NULL,
    target      text,
    metadata    jsonb       NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- The only read pattern: a tenant's trail, newest first, paginated.
CREATE INDEX audit_events_tenant_created_idx
    ON audit_events (tenant_id, created_at DESC, id DESC);
