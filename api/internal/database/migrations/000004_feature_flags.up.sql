-- Per-tenant feature flags: a small jsonb map of flag key → bool override. The
-- catalog (which flags exist, names, defaults) lives in code; this stores only a
-- tenant's deviations from the defaults. Gates opt-in capabilities (e.g. GitHub
-- personal-access-token auth) per workspace.
ALTER TABLE tenants ADD COLUMN feature_flags jsonb NOT NULL DEFAULT '{}';
