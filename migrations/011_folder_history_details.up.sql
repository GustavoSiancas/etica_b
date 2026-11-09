BEGIN;
SELECT pg_advisory_xact_lock(8020, 11);
DO $upgrade$
BEGIN
 IF EXISTS (SELECT 1 FROM app_schema_migrations WHERE version='011_folder_history_details') THEN RETURN; END IF;
 ALTER TABLE expediente_history
  ADD COLUMN location VARCHAR(30),
  ADD COLUMN content_snapshot JSONB,
  ADD CONSTRAINT history_location_check CHECK (location IN ('DIRECCION DE ETICA','CONSEJO DE ETICA','TRIBUNAL DE HONOR')),
  ADD CONSTRAINT history_content_snapshot_check CHECK (content_snapshot IS NULL OR (folder_id IS NOT NULL AND jsonb_typeof(content_snapshot)='object'));
 CREATE INDEX idx_folder_history_timeline ON expediente_history(expediente_id,folder_id,changed_at DESC,id DESC);
 -- Past events retain NULL: the current content cannot reconstruct past snapshots.
 INSERT INTO app_schema_migrations(version) VALUES ('011_folder_history_details');
END;
$upgrade$;
COMMIT;
