BEGIN;
SELECT pg_advisory_xact_lock(8020, 6);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='006_folder_opening_date') THEN
        RETURN;
    END IF;
    LOCK TABLE expedientes, expediente_folders IN SHARE ROW EXCLUSIVE MODE;
    ALTER TABLE expediente_folders ADD COLUMN opening_date TIMESTAMPTZ;
    -- A calendar date is interpreted as midnight in Peru, regardless of session timezone.
    UPDATE expediente_folders f
    SET opening_date = CASE WHEN t.name = 'EXPEDIENTE PRINCIPAL'
        THEN e.opening_date::timestamp AT TIME ZONE 'America/Lima'
        ELSE f.created_at END
    FROM expedientes e, folder_types t
    WHERE e.id=f.expediente_id AND t.id=f.folder_type_id;
    EXECUTE $function$
        CREATE OR REPLACE FUNCTION create_principal_folder() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN
            INSERT INTO expediente_folders(expediente_id, folder_type_id, opening_date)
            SELECT NEW.id, id, NEW.opening_date::timestamp AT TIME ZONE 'America/Lima'
            FROM folder_types WHERE name='EXPEDIENTE PRINCIPAL';
            RETURN NEW;
        END;
        $$;
    $function$;
    INSERT INTO app_schema_migrations(version) VALUES ('006_folder_opening_date');
END;
$upgrade$;
COMMIT;
