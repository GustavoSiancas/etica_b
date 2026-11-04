BEGIN;
SELECT pg_advisory_xact_lock(8020, 7);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='007_folder_type_abbreviation') THEN
        RETURN;
    END IF;
    ALTER TABLE folder_types ADD COLUMN abbreviation VARCHAR(20);
    UPDATE folder_types SET abbreviation = CASE name
        WHEN 'EXPEDIENTE PRINCIPAL' THEN 'EP'
        WHEN 'EXPEDIENTE CAUTELAR' THEN 'MC'
    END
    WHERE name IN ('EXPEDIENTE PRINCIPAL', 'EXPEDIENTE CAUTELAR');
    INSERT INTO app_schema_migrations(version) VALUES ('007_folder_type_abbreviation');
END;
$upgrade$;
COMMIT;
