-- Upgrade an existing installation; safe to execute again.
BEGIN;
SELECT pg_advisory_xact_lock(8020, 2);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='002_expediente_state_location') THEN
        RETURN;
    END IF;
    -- Do not silently discard unrecognized custom states.
    IF EXISTS (SELECT 1 FROM expedientes WHERE status IS NULL OR
        upper(btrim(status)) NOT IN ('OPEN','IN_PROGRESS','CLOSED','ABIERTO','CERRADO')) THEN
        RAISE EXCEPTION 'Resolve custom expediente statuses before applying migration 002'
            USING ERRCODE='23514';
    END IF;
    ALTER TABLE expedientes DROP CONSTRAINT IF EXISTS expedientes_status_check;
    ALTER TABLE expedientes DROP CONSTRAINT IF EXISTS expedientes_location_check;
    ALTER TABLE expedientes ADD COLUMN IF NOT EXISTS location VARCHAR(30);
    UPDATE expedientes SET status = CASE upper(btrim(status))
        WHEN 'OPEN' THEN 'ABIERTO'
        WHEN 'IN_PROGRESS' THEN 'ABIERTO'
        WHEN 'CLOSED' THEN 'CERRADO'
        ELSE upper(btrim(status)) END
    WHERE status NOT IN ('ABIERTO','CERRADO');
    UPDATE expedientes SET location='DIRECCION DE ETICA' WHERE location IS NULL;
    ALTER TABLE expedientes ALTER COLUMN status SET DEFAULT 'ABIERTO';
    ALTER TABLE expedientes ALTER COLUMN status SET NOT NULL;
    ALTER TABLE expedientes ALTER COLUMN location SET DEFAULT 'DIRECCION DE ETICA';
    ALTER TABLE expedientes ALTER COLUMN location SET NOT NULL;
    ALTER TABLE expedientes ADD CONSTRAINT expedientes_status_check CHECK (status IN ('ABIERTO','CERRADO'));
    ALTER TABLE expedientes ADD CONSTRAINT expedientes_location_check
        CHECK (location IN ('DIRECCION DE ETICA','CONSEJO DE ETICA','TRIBUNAL DE HONOR'));
    INSERT INTO app_schema_migrations(version) VALUES ('002_expediente_state_location');
END;
$upgrade$;
COMMIT;
