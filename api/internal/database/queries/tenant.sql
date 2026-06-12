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
-- Region is write-once (ADR-0014), enforced atomically: the WHERE on DO UPDATE
-- only applies when the stored region is unset or already equals the requested
-- one. A conflicting region skips the update, so RETURNING yields no row — the
-- caller maps that to ErrRegionImmutable. Race-safe with no partial write.
-- onboarded_at is stamped once, then preserved.
INSERT INTO tenants (zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at, updated_at)
VALUES (@zitadel_org_id, @name, @size, @region, @use_cases, NULLIF(@use_case_other::text, ''), now(), now())
ON CONFLICT (zitadel_org_id) DO UPDATE SET
    name           = EXCLUDED.name,
    size           = EXCLUDED.size,
    region         = COALESCE(tenants.region, EXCLUDED.region),
    use_cases      = EXCLUDED.use_cases,
    use_case_other = EXCLUDED.use_case_other,
    onboarded_at   = COALESCE(tenants.onboarded_at, now()),
    updated_at     = now()
WHERE tenants.region IS NULL OR tenants.region = EXCLUDED.region
RETURNING id, zitadel_org_id, name, size, region, use_cases, use_case_other, onboarded_at, created_at, updated_at;
