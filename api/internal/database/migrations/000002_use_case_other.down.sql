-- Down for 000002 — drop the free-text "other" use case column.
ALTER TABLE tenants DROP COLUMN IF EXISTS use_case_other;
