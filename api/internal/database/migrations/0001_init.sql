-- 0001 — tenant model (design-doc 0005).
-- A tenant is one ZITADEL organization, 1:1. The row is created lazily on the
-- first authenticated request and filled in by onboarding. region is set once
-- at onboarding and is immutable thereafter (enforced in the app + by COALESCE
-- on update).
CREATE TABLE IF NOT EXISTS tenants (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    zitadel_org_id text        UNIQUE NOT NULL,
    name           text,
    size           text,
    region         text,
    use_cases      jsonb       NOT NULL DEFAULT '[]',
    onboarded_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
