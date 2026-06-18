-- When the per-unit total_estimate was last (re)computed (design-doc 0012). Set to
-- now() every time discovery stores a fresh estimate (connector.EstimateTotals), so
-- the UI can show how stale the synced/total ceiling is. Nullable — NULL means the
-- total was never checked (e.g. discovery never reached this unit, or the connector
-- doesn't estimate).
ALTER TABLE sync_unit ADD COLUMN total_estimate_at timestamptz;
