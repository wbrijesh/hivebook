-- Tenant queries (design-doc 0005). Every Go-API SQL statement lives here and is
-- consumed via the generated package (internal/database/gen); nothing else writes SQL.

-- name: GetTenantByOrg :one
SELECT id, zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at, created_at, updated_at
FROM tenants
WHERE zitadel_org_id = $1;

-- name: CreateTenantIfAbsent :exec
INSERT INTO tenants (zitadel_org_id)
VALUES ($1)
ON CONFLICT (zitadel_org_id) DO NOTHING;

-- name: CompleteOnboarding :one
-- Region is write-once: COALESCE keeps an already-set region. The app rejects a
-- *changing* region before calling this (a COALESCE alone would silently ignore it).
-- onboarded_at is stamped once, then preserved.
INSERT INTO tenants (zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), now(), now())
ON CONFLICT (zitadel_org_id) DO UPDATE SET
    name           = EXCLUDED.name,
    size           = EXCLUDED.size,
    region         = COALESCE(tenants.region, EXCLUDED.region),
    use_cases      = EXCLUDED.use_cases,
    use_case_other = EXCLUDED.use_case_other,
    onboarded_at   = COALESCE(tenants.onboarded_at, now()),
    updated_at     = now()
RETURNING id, zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at, created_at, updated_at;
