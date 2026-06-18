-- Baseline schema for the integrations service on the Temporal sync architecture
-- (design-doc 0012). Orchestration lives in Temporal; Postgres holds the durable
-- connector state — connections, the connector-defined sync units (cursor +
-- discovery lifecycle), the raw-artifact manifest (with the erasure outbox), the
-- sync-run lineage, and the adaptive admission buckets. There is no River, no
-- enqueuer scan, and no inflight counter — those scaled O(units) and are gone.

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- gen_random_uuid()

-- connector_connection — one row per OAuth/App connection. Lifecycle (schedule,
-- trigger, retry) now lives in the Temporal ConnectionWorkflow, so the River-era
-- orchestration columns (inflight_jobs, next_sync_at, sync_requested_at) are gone.
CREATE TABLE connector_connection (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            text        NOT NULL,
    region               text        NOT NULL,
    connector_id         text        NOT NULL,
    status               text        NOT NULL DEFAULT 'connected',
    account              text        NOT NULL DEFAULT '',
    access_token         bytea       NOT NULL,
    refresh_token        bytea,
    token_expiry         timestamptz,
    scopes               text        NOT NULL DEFAULT '',
    installation_id      text        NOT NULL DEFAULT '',
    external_id          text        NOT NULL DEFAULT '',
    last_error           text        NOT NULL DEFAULT '',
    last_attempt_at      timestamptz,
    last_success_at      timestamptz,
    consecutive_failures int         NOT NULL DEFAULT 0,
    deleted_at           timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    -- One connection per (tenant, connector, external account/installation). The
    -- external_id disambiguates two installations of the same connector.
    UNIQUE (tenant_id, connector_id, external_id)
);

CREATE INDEX idx_connector_connection_tenant ON connector_connection (tenant_id);

-- sync_unit — the connector-defined unit of sync (ADR-0035): a GitHub repo, a Jira
-- project, a Slack channel, or the single Drive change-feed. Replaces the old
-- project + watermark tables: the cursor (the high-water mark, ADR-0031) and the
-- discovery lifecycle (mark-and-sweep, ADR-0037) live here, per unit. The workflow
-- drains this table in batches and never holds the unit list (design-doc 0012).
CREATE TABLE sync_unit (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_id uuid        NOT NULL REFERENCES connector_connection (id) ON DELETE CASCADE,
    external_id   text        NOT NULL, -- the connector's unit id (repo full-name, "drive", …)
    name          text        NOT NULL DEFAULT '',
    kind          text        NOT NULL DEFAULT '',
    selected      boolean     NOT NULL DEFAULT true,
    cursor        text        NOT NULL DEFAULT '',  -- opaque connector checkpoint (ADR-0031/0038)
    status        text        NOT NULL DEFAULT 'idle',
    generation    bigint      NOT NULL DEFAULT 0,   -- mark-and-sweep generation (ADR-0037)
    last_seen_at  timestamptz,                      -- last discovery pass that observed this unit
    last_error    text        NOT NULL DEFAULT '',
    next_retry_at timestamptz,                      -- poison-unit exponential backoff
    committed_at  timestamptz,                      -- first full pass done → artifacts visible (ADR-0033)
    artifact_count int        NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connection_id, external_id)
);

-- raw_artifact — the manifest indexing the raw bytes in object storage (design-doc
-- 0008/0009). sync_container_id is the SYNC unit's external_id (the visibility join
-- key); container_id is the per-artifact manifest container (they coincide for
-- GitHub, differ for Drive: drive vs folder). purge_pending/purge_deadline are the
-- erasure deletion outbox (ADR-0039): the row is held until the object delete
-- confirms; deleted_at is the soft-delete that hides it meanwhile.
CREATE TABLE raw_artifact (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          text        NOT NULL,
    region             text        NOT NULL,
    connection_id      uuid        NOT NULL REFERENCES connector_connection (id) ON DELETE CASCADE,
    source             text        NOT NULL,
    source_native_kind text        NOT NULL DEFAULT '',
    container_id       text        NOT NULL DEFAULT '',
    sync_container_id  text        NOT NULL DEFAULT '',
    container_kind     text        NOT NULL DEFAULT '',
    container_name     text        NOT NULL DEFAULT '',
    external_id        text        NOT NULL,
    source_url         text        NOT NULL DEFAULT '',
    object_key         text        NOT NULL,
    content_hash       text        NOT NULL DEFAULT '',
    source_created_at  timestamptz,
    source_updated_at  timestamptz,
    ingested_at        timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,                       -- soft-delete (hidden, pending purge)
    purge_pending      boolean     NOT NULL DEFAULT false, -- erasure outbox: object delete owed (ADR-0039)
    purge_deadline     timestamptz,                        -- erasure SLA — past this, page
    UNIQUE (connection_id, external_id)
);

CREATE INDEX idx_raw_artifact_tenant_ingested ON raw_artifact (tenant_id, ingested_at DESC);
CREATE INDEX idx_raw_artifact_connection ON raw_artifact (connection_id);
CREATE INDEX idx_raw_artifact_sync_container ON raw_artifact (connection_id, sync_container_id);
CREATE INDEX idx_raw_artifact_deleted ON raw_artifact (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_raw_artifact_purge_pending ON raw_artifact (purge_deadline) WHERE purge_pending;

-- sync_run — per-connection/per-unit run lineage (design-doc 0012): the on-demand
-- detail behind the aggregate SLIs. unit_id is the sync_unit external_id the run
-- covered (renamed from container_id; '' for a whole-connection run).
CREATE TABLE sync_run (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     text        NOT NULL,
    connection_id uuid        NOT NULL REFERENCES connector_connection (id) ON DELETE CASCADE,
    connector_id  text        NOT NULL,
    unit_id       text        NOT NULL DEFAULT '',
    result        text        NOT NULL,
    artifacts     int         NOT NULL DEFAULT 0,
    error         text        NOT NULL DEFAULT '',
    started_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_sync_run_connection ON sync_run (connection_id, started_at DESC);
CREATE INDEX idx_sync_run_finished ON sync_run (finished_at);

-- admission_bucket — the adaptive admission controller's state (ADR-0036): one row
-- per limiter scope. 'credential' scopes a connection's API budget (the adaptive
-- AIMD limit L); 'tenant' scopes a tenant's hard in-flight cap. The acquire/release
-- + AIMD logic is Phase 3; this is the table and its basic CRUD.
CREATE TABLE admission_bucket (
    scope_type       text        NOT NULL, -- 'credential' | 'tenant'
    scope_key        text        NOT NULL, -- connection_id or tenant_id
    connector_id     text        NOT NULL DEFAULT '',
    concurrency_limit int        NOT NULL, -- the adaptive AIMD limit L
    in_flight        int         NOT NULL DEFAULT 0,
    cooldown_until   timestamptz,          -- Retry-After backoff window
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_type, scope_key)
);
