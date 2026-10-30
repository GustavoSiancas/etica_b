-- Number future expedientes using opening_date; keep all existing numbers.
BEGIN;
SELECT pg_advisory_xact_lock(8020, 3);
CREATE TABLE IF NOT EXISTS app_schema_migrations (
    version VARCHAR(100) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
DO $upgrade$
BEGIN
    IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='003_expediente_opening_year') THEN
        RETURN;
    END IF;
    LOCK TABLE expedientes IN SHARE ROW EXCLUSIVE MODE;
    LOCK TABLE expediente_year_counters IN SHARE ROW EXCLUSIVE MODE;
    -- Honor already allocated numbers, even when a counter was imported/reset.
    INSERT INTO expediente_year_counters(year,last_number)
    SELECT split_part(number,'-',2)::INTEGER, MAX(split_part(number,'-',1)::BIGINT)
    FROM expedientes
    WHERE number ~ '^[1-9][0-9]*-[1-9][0-9]{0,3}$'
    GROUP BY split_part(number,'-',2)::INTEGER
    ON CONFLICT(year) DO UPDATE
        SET last_number=GREATEST(expediente_year_counters.last_number,EXCLUDED.last_number);
    EXECUTE $function$
        CREATE OR REPLACE FUNCTION assign_expediente_code() RETURNS trigger LANGUAGE plpgsql AS $$
        BEGIN
            IF NEW.opening_date IS NULL THEN
                RAISE EXCEPTION 'opening_date is required to assign expediente number' USING ERRCODE='23502';
            END IF;
            NEW.number := next_expediente_code(EXTRACT(YEAR FROM NEW.opening_date)::INTEGER);
            RETURN NEW;
        END;
        $$;
    $function$;
    INSERT INTO app_schema_migrations(version) VALUES ('003_expediente_opening_year');
END;
$upgrade$;
COMMIT;
