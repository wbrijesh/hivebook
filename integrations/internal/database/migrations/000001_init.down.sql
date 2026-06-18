-- Drop the integrations baseline schema (children first; FKs cascade regardless).
DROP TABLE IF EXISTS admission_bucket;
DROP TABLE IF EXISTS sync_run;
DROP TABLE IF EXISTS raw_artifact;
DROP TABLE IF EXISTS sync_unit;
DROP TABLE IF EXISTS connector_connection;
