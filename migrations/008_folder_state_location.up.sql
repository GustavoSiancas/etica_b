BEGIN;
SELECT pg_advisory_xact_lock(8020, 8);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='008_folder_state_location') THEN
        RETURN;
    END IF;
    LOCK TABLE expedientes, expediente_folders IN SHARE ROW EXCLUSIVE MODE;
    ALTER TABLE expediente_folders
        ADD COLUMN status VARCHAR(30) NOT NULL DEFAULT 'ABIERTO',
        ADD COLUMN location VARCHAR(30) NOT NULL DEFAULT 'DIRECCION DE ETICA',
        ADD CONSTRAINT folder_status_check CHECK (status IN ('ABIERTO','CERRADO')),
        ADD CONSTRAINT folder_location_check CHECK (location IN ('DIRECCION DE ETICA','CONSEJO DE ETICA','TRIBUNAL DE HONOR'));
    -- Ensure every expediente has a principal before removing its legacy fields.
    INSERT INTO expediente_folders(expediente_id,folder_type_id,opening_date)
    SELECT e.id,t.id,e.opening_date::timestamp AT TIME ZONE 'America/Lima'
    FROM expedientes e CROSS JOIN folder_types t WHERE t.name='EXPEDIENTE PRINCIPAL'
    ON CONFLICT (expediente_id,folder_type_id) DO NOTHING;
    IF EXISTS (SELECT 1 FROM expedientes e WHERE NOT EXISTS (
        SELECT 1 FROM expediente_folders f JOIN folder_types t ON t.id=f.folder_type_id
        WHERE f.expediente_id=e.id AND t.name='EXPEDIENTE PRINCIPAL'
    )) THEN
        RAISE EXCEPTION 'Every expediente needs an EXPEDIENTE PRINCIPAL folder before migration 008';
    END IF;
    UPDATE expediente_folders f SET status=e.status, location=e.location
    FROM expedientes e,folder_types t
    WHERE f.expediente_id=e.id AND f.folder_type_id=t.id AND t.name='EXPEDIENTE PRINCIPAL';
    ALTER TABLE expedientes DROP COLUMN status, DROP COLUMN location;
    CREATE INDEX idx_folder_state_location ON expediente_folders(status,location,expediente_id);
    INSERT INTO app_schema_migrations(version) VALUES ('008_folder_state_location');
END;
$upgrade$;
COMMIT;
