-- When the connection's catalog (its sync_units) was first discovered (design-doc
-- 0012). Set to now() the first time a healthy, non-degraded discovery completes;
-- thereafter re-stamped every healthy pass. NULL means discovery has not finished
-- yet — the post-connect cold-start window (KEDA worker spin-up) before the first
-- DiscoverUnits runs. The UI shows a "discovering…" loading state while NULL, so an
-- empty unit list before discovery never reads as a false "no projects".
ALTER TABLE connector_connection ADD COLUMN catalog_discovered_at timestamptz;
