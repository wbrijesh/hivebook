-- Per-connection auto-sync cadence, user-configurable (design-doc 0012). The
-- ConnectionWorkflow reads these on every idle entry (via GetSyncSchedule) so a
-- change applies live and survives continueAsNew:
--
-- 1. `auto_sync_enabled` gates the periodic re-sync timer. When false the workflow
--    arms no timer — it only runs on a manual TriggerSync (or other signals).
-- 2. `sync_interval_seconds` is the re-sync cadence in seconds.
--
-- Defaults preserve today's behaviour: auto-sync on, every 15 minutes (900s).
ALTER TABLE connector_connection ADD COLUMN auto_sync_enabled boolean NOT NULL DEFAULT true;
ALTER TABLE connector_connection ADD COLUMN sync_interval_seconds integer NOT NULL DEFAULT 900;
