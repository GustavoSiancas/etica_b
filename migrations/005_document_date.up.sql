BEGIN;
SELECT pg_advisory_xact_lock(8020, 5);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='005_document_date') THEN
        RETURN;
    END IF;
    ALTER TABLE documents ADD COLUMN IF NOT EXISTS document_date TIMESTAMPTZ;
    INSERT INTO app_schema_migrations(version) VALUES ('005_document_date');
END;
$upgrade$;
COMMIT;
