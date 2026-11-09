BEGIN;
SELECT pg_advisory_xact_lock(8020, 9);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='009_expediente_status_closing_dates') THEN
        RETURN;
    END IF;
    LOCK TABLE expedientes, expediente_folders IN SHARE ROW EXCLUSIVE MODE;
    ALTER TABLE expedientes
        ADD COLUMN status VARCHAR(30) NOT NULL DEFAULT 'ABIERTO',
        ADD COLUMN closing_date DATE,
        ADD CONSTRAINT expedientes_status_check CHECK (status IN ('ABIERTO','CERRADO'));
    -- Preserve opening dates and use their existing types for closing dates.
    -- Existing records have no known closing date, so leave it NULL.
    ALTER TABLE expediente_folders ADD COLUMN closing_date TIMESTAMPTZ;
    INSERT INTO app_schema_migrations(version) VALUES ('009_expediente_status_closing_dates');
END;
$upgrade$;
COMMIT;
