-- Per-unit total-item estimate (design-doc 0012, live synced/total progress). A
-- best-effort count fetched up front at discovery (connector.EstimateTotals) so the
-- UI can show synced/total per project and sort descending by total. Defaults to 0
-- ("unknown") — an absent estimate never blocks discovery or sync.
ALTER TABLE sync_unit ADD COLUMN total_estimate int NOT NULL DEFAULT 0;
