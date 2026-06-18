ALTER TABLE connector_connection DROP COLUMN onboarded;
ALTER TABLE sync_unit ALTER COLUMN selected SET DEFAULT true;
