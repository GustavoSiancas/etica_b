-- DATOS COMPLETAMENTE FICTICIOS PARA DESARROLLO Y PRUEBAS.
-- Ejecutar después de migrations/001_initial.up.sql en una base de pruebas vacía.
-- Replaces [DEMO] records and internally marked fixture participants; repeatable.
-- Attachments use verified external public PDF URLs for UI testing; not R2 objects.

BEGIN;

-- Internal fixture ownership; display names do not need a [DEMO] prefix.
ALTER TABLE denunciantes ADD COLUMN IF NOT EXISTS is_demo BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE denunciados ADD COLUMN IF NOT EXISTS is_demo BOOLEAN NOT NULL DEFAULT FALSE;

-- Permite que el reset retire solamente eventos de historial creados por este fixture.
-- El resto del historial conserva su protección append-only.
CREATE OR REPLACE FUNCTION protect_participant_history() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE'
       AND current_setting('app.demo_reset', TRUE) = 'on'
       AND OLD.reason = 'Registro inicial de datos ficticios para pruebas.' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'Participant history is append-only';
END;
$$;

SET LOCAL app.demo_reset = 'on';

-- Restore missing catalogs, including after DELETE /dev/database/clear.
INSERT INTO document_stages (code, name, sort_order) VALUES
    ('CALIFICACION_DENUNCIA', 'Calificación de la denuncia.', 1),
    ('DESCARGOS_INVESTIGACION', 'Descargos e investigación.', 2),
    ('PROGRAMACION_AUDIENCIA', 'Programación de audiencia.', 3),
    ('AUDIENCIA', 'Audiencia.', 4),
    ('RESOLUCION_FINAL', 'Resolución final.', 5)
ON CONFLICT (code) DO NOTHING;

INSERT INTO document_substages (stage_id, code, name, sort_order)
SELECT s.id, v.code, v.name, v.sort_order
FROM (VALUES
    ('CALIFICACION_DENUNCIA','CALIFICACION','Calificación de la denuncia.',1),
    ('CALIFICACION_DENUNCIA','PROYECCION','Proyección de la calificación.',2),
    ('CALIFICACION_DENUNCIA','REVISION_FIRMA','Revisión y firma de la resolución.',3),
    ('CALIFICACION_DENUNCIA','NOTIFICACION_RESOLUCION','Notificación de resolución.',4),
    ('CALIFICACION_DENUNCIA','CARGOS_NOTIFICACION','Cargos de la notificación.',5),
    ('DESCARGOS_INVESTIGACION','DESCARGOS_ESCRITOS','Descargos y escritos del denunciante o denunciado.',1),
    ('DESCARGOS_INVESTIGACION','FORMALIZACION_ESCRITOS','Formalización de escritos del denunciante o denunciado.',2),
    ('PROGRAMACION_AUDIENCIA','PROGRAMACION','Programación de audiencia.',1),
    ('PROGRAMACION_AUDIENCIA','NOTIFICACION_AUDIENCIA','Notificación de audiencia.',2),
    ('PROGRAMACION_AUDIENCIA','CARGOS_AUDIENCIA','Cargos de notificación.',3),
    ('AUDIENCIA','ACTA_AUDIENCIA','Acta de audiencia.',1),
    ('RESOLUCION_FINAL','FICHA_VOTACION','Ficha de votación.',1),
    ('RESOLUCION_FINAL','RESOLUCION','Resolución final.',2),
    ('RESOLUCION_FINAL','NOTIFICACION_FINAL','Notificación de la resolución.',3),
    ('RESOLUCION_FINAL','CARGOS_FINAL','Cargos de la notificación.',4)
) AS v(stage_code, code, name, sort_order)
JOIN document_stages s ON s.code = v.stage_code
ON CONFLICT (code) DO NOTHING;


DELETE FROM expediente_participant_history
WHERE reason = 'Registro inicial de datos ficticios para pruebas.'
  AND expediente_id IN (SELECT id FROM expedientes WHERE name LIKE '[DEMO]%');

DELETE FROM document_denunciantes dd USING documents d, expedientes e
WHERE dd.document_id = d.id AND d.expediente_id = e.id AND e.name LIKE '[DEMO]%';
DELETE FROM document_denunciados dd USING documents d, expedientes e
WHERE dd.document_id = d.id AND d.expediente_id = e.id AND e.name LIKE '[DEMO]%';
DELETE FROM documents d USING expedientes e
WHERE d.expediente_id = e.id AND e.name LIKE '[DEMO]%';

DELETE FROM escritos WHERE title LIKE '[DEMO]%';
DELETE FROM expediente_denunciantes ep USING expedientes e
WHERE ep.expediente_id = e.id AND e.name LIKE '[DEMO]%';
DELETE FROM expediente_denunciados ep USING expedientes e
WHERE ep.expediente_id = e.id AND e.name LIKE '[DEMO]%';
DELETE FROM denunciantes WHERE is_demo OR name LIKE '[DEMO]%';
DELETE FROM denunciados WHERE is_demo OR name LIKE '[DEMO]%';
DELETE FROM expedientes WHERE name LIKE '[DEMO]%';

INSERT INTO expedientes (name, opening_date, status, init_type, created_at, updated_at) VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', '2025-02-14', 'IN_PROGRESS', 'DENUNCIA_DE_PARTE', '2025-02-14 09:15:00-05', '2025-03-20 16:30:00-05'),
    ('[DEMO] Uso de recursos institucionales', '2025-06-03', 'OPEN', 'DENUNCIA_DE_OFICIO', '2025-06-03 10:00:00-05', '2025-06-03 10:00:00-05'),
    ('[DEMO] Incumplimiento de deberes funcionales', '2026-01-19', 'CLOSED', 'COMUNICACIONES_DE_ENTIDADES_PUBLICAS', '2026-01-19 08:45:00-05', '2026-05-22 17:20:00-05');

INSERT INTO denunciantes (name, created_at, updated_at, is_demo) VALUES
    ('Valeria Rojas Salazar', '2025-02-14 09:10:00-05', '2025-02-14 09:10:00-05', TRUE),
    ('Martín Paredes León', '2025-02-14 09:12:00-05', '2025-02-14 09:12:00-05', TRUE),
    ('Inversiones Valle Claro S.A.C.', '2025-06-03 09:55:00-05', '2025-06-03 09:55:00-05', TRUE),
    ('Diego Cárdenas Ruiz', '2026-01-19 08:40:00-05', '2026-01-19 08:40:00-05', TRUE);

INSERT INTO denunciados (name, created_at, updated_at, is_demo) VALUES
    ('Andrés Quiroz Vega CAL 20016', '2025-02-14 09:11:00-05', '2025-02-14 09:11:00-05', TRUE),
    ('Lucía Benavides Cruz CAL 20017', '2025-02-14 09:13:00-05', '2025-02-14 09:13:00-05', TRUE),
    ('Sergio Valdivia Ponce CAL 20018', '2025-06-03 09:56:00-05', '2025-06-03 09:56:00-05', TRUE),
    ('Patricia Salas Méndez CAL 20019', '2026-01-19 08:41:00-05', '2026-01-19 08:41:00-05', TRUE);

-- Additional fictional people with realistic names and simulated CAL codes.
WITH names AS (
    SELECT ARRAY['Alejandro','Camila','Fernando','Gabriela','Javier','Mariana','Nicolás','Paola','Ricardo','Sofía','Antonio','Daniela','Eduardo','Florencia','Gonzalo','Isabel','Joaquín','Lorena','Miguel','Natalia','Óscar','Pilar','Renato','Silvia','Tomás'] AS first_names,
           ARRAY['Aguilar','Benavides','Castillo','Delgado','Espinoza','Flores','García','Herrera','Ibáñez','Jiménez','López','Mendoza','Navarro','Ortega','Paredes'] AS last_names,
           ARRAY['Salazar','Torres','Vargas','Ramos','Quispe','Medina','Rojas','Cabrera','Soto','Vega','Campos','Reyes','Ponce','Ruiz','León','Cruz','Huamán'] AS second_names
)
INSERT INTO denunciantes (name, is_demo)
SELECT CASE WHEN n <= 75 THEN
       first_names[1 + ((n - 1) % 25)] || ' ' ||
       last_names[1 + (((n - 1) / 25) % 15)] || ' ' ||
       second_names[1 + ((n - 1) % 17)]
       ELSE (ARRAY['Inversiones','Servicios Integrales','Comercial','Constructora','Consultora'])
            [1 + ((n - 76) % 5)] || ' ' ||
            (ARRAY['Valle Azul','Costa Verde','Horizonte Sur','Sierra Clara','Puerto Nuevo'])
            [1 + ((n - 76) / 5)] || ' ' ||
            (ARRAY['S.A.C.','S.R.L.','E.I.R.L.'])[1 + ((n - 76) % 3)]
       END, TRUE
FROM names CROSS JOIN generate_series(1, 100) AS series(n);

-- Additional fictional people with realistic names and simulated CAL codes.
WITH names AS (
    SELECT ARRAY['Alejandro','Camila','Fernando','Gabriela','Javier','Mariana','Nicolás','Paola','Ricardo','Sofía','Antonio','Daniela','Eduardo','Florencia','Gonzalo','Isabel','Joaquín','Lorena','Miguel','Natalia','Óscar','Pilar','Renato','Silvia','Tomás'] AS first_names,
           ARRAY['Aguilar','Benavides','Castillo','Delgado','Espinoza','Flores','García','Herrera','Ibáñez','Jiménez','López','Mendoza','Navarro','Ortega','Paredes'] AS last_names,
           ARRAY['Salazar','Torres','Vargas','Ramos','Quispe','Medina','Rojas','Cabrera','Soto','Vega','Campos','Reyes','Ponce','Ruiz','León','Cruz','Huamán'] AS second_names
)
INSERT INTO denunciados (name, is_demo)
SELECT first_names[1 + ((n - 1) % 25)] || ' ' ||
       last_names[1 + (((n - 1) / 25) % 15)] || ' ' ||
       second_names[1 + ((n - 1) % 17)] || ' CAL ' || (20200 + n)::text, TRUE
FROM names CROSS JOIN generate_series(1, 150) AS series(n);

INSERT INTO expediente_denunciantes (expediente_id, denunciante_id, is_active)
SELECT e.id, p.id, TRUE
FROM (VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', 'Valeria Rojas Salazar'),
    ('[DEMO] Compra de suministros y conflicto de interés', 'Martín Paredes León'),
    ('[DEMO] Uso de recursos institucionales', 'Inversiones Valle Claro S.A.C.'),
    ('[DEMO] Incumplimiento de deberes funcionales', 'Diego Cárdenas Ruiz')
) AS v(expediente_name, person_name)
JOIN expedientes e ON e.name = v.expediente_name
JOIN denunciantes p ON p.name = v.person_name;

INSERT INTO expediente_denunciados (expediente_id, denunciado_id, is_active)
SELECT e.id, p.id, TRUE
FROM (VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', 'Andrés Quiroz Vega CAL 20016'),
    ('[DEMO] Compra de suministros y conflicto de interés', 'Lucía Benavides Cruz CAL 20017'),
    ('[DEMO] Uso de recursos institucionales', 'Sergio Valdivia Ponce CAL 20018'),
    ('[DEMO] Incumplimiento de deberes funcionales', 'Patricia Salas Méndez CAL 20019')
) AS v(expediente_name, person_name)
JOIN expedientes e ON e.name = v.expediente_name
JOIN denunciados p ON p.name = v.person_name;

INSERT INTO expediente_participant_history
    (expediente_id, participant_type, participant_id, action, reason, created_at, new_name)
SELECT e.id, 'DENUNCIANTE', p.id, 'ADDED', 'Registro inicial de datos ficticios para pruebas.', e.created_at, p.name
FROM expediente_denunciantes ep
JOIN expedientes e ON e.id = ep.expediente_id AND e.name LIKE '[DEMO]%'
JOIN denunciantes p ON p.id = ep.denunciante_id;

INSERT INTO expediente_participant_history
    (expediente_id, participant_type, participant_id, action, reason, created_at, new_name)
SELECT e.id, 'DENUNCIADO', p.id, 'ADDED', 'Registro inicial de datos ficticios para pruebas.', e.created_at, p.name
FROM expediente_denunciados ep
JOIN expedientes e ON e.id = ep.expediente_id AND e.name LIKE '[DEMO]%'
JOIN denunciados p ON p.id = ep.denunciado_id;

-- One document per active substage for each fixture expediente.
-- External public PDFs are sample attachments, unrelated to the fictional cases.
INSERT INTO documents (expediente_id, name, file_url, document_date, substage_id, status, created_at, updated_at)
SELECT e.id, '[DEMO] ' || ss.name,
       (ARRAY['https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf','https://www.adobe.com/content/dam/cc1/en/accessibility/products/acrobat/pdfs/acrobat-x-pdf-accessibility-best-practices.pdf','https://wwwimages2.adobe.com/content/dam/cc/en/accessibility/pdfs/accessing-pdf-sr.pdf'])[1 + ((row_number() OVER (ORDER BY e.id, st.sort_order, ss.sort_order, ss.id) - 1) % 3)::integer],
       e.opening_date + ss.sort_order, ss.id,
       CASE (ss.sort_order % 4) WHEN 0 THEN 'DRAFT' WHEN 1 THEN 'APPROVED' WHEN 2 THEN 'PENDING' ELSE 'REJECTED' END,
       e.created_at, e.created_at
FROM expedientes e
CROSS JOIN document_substages ss
JOIN document_stages st ON st.id = ss.stage_id
WHERE e.name LIKE '[DEMO]%' AND ss.is_active AND st.is_active;

-- Las relaciones de documentos incluyen automáticamente a cada participante activo del expediente.
INSERT INTO document_denunciantes (document_id, expediente_denunciante_id, expediente_id)
SELECT d.id, ep.id, d.expediente_id
FROM documents d
JOIN expedientes e ON e.id = d.expediente_id AND e.name LIKE '[DEMO]%'
JOIN expediente_denunciantes ep ON ep.expediente_id = d.expediente_id AND ep.is_active;

INSERT INTO document_denunciados (document_id, expediente_denunciado_id, expediente_id)
SELECT d.id, ep.id, d.expediente_id
FROM documents d
JOIN expedientes e ON e.id = d.expediente_id AND e.name LIKE '[DEMO]%'
JOIN expediente_denunciados ep ON ep.expediente_id = d.expediente_id AND ep.is_active;

WITH demo_escritos(expediente_name, title, type, document_date, status, created_at, sender_type, sender_name) AS (VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Escrito de solicitud de acceso al expediente', 'Solicitud de acceso', '2025-02-18'::date, 'ATENDIDA', '2025-02-18 10:05:00-05'::timestamptz, 'DENUNCIANTE', 'Valeria Rojas Salazar'),
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Escrito de descargos y anexos', 'Descargos', '2025-03-05'::date, 'ATENDIDA', '2025-03-05 12:30:00-05'::timestamptz, 'DENUNCIADO', 'Andrés Quiroz Vega CAL 20016'),
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Escrito de ampliación de denuncia', 'Ampliación de denuncia', '2025-03-08'::date, 'NUEVA', '2025-03-08 09:40:00-05'::timestamptz, 'DENUNCIANTE', 'Martín Paredes León'),
    ('[DEMO] Uso de recursos institucionales', '[DEMO] Escrito de entrega de información', 'Entrega de información', '2025-06-12'::date, 'ATENDIDA', '2025-06-12 11:15:00-05'::timestamptz, 'DENUNCIANTE', 'Inversiones Valle Claro S.A.C.'),
    ('[DEMO] Uso de recursos institucionales', '[DEMO] Escrito de respuesta a requerimiento', 'Respuesta a requerimiento', '2025-06-20'::date, 'NUEVA', '2025-06-20 16:10:00-05'::timestamptz, 'DENUNCIADO', 'Sergio Valdivia Ponce CAL 20018'),
    ('[DEMO] Incumplimiento de deberes funcionales', '[DEMO] Escrito de observaciones a la resolución', 'Observaciones', '2026-04-02'::date, 'ATENDIDA', '2026-04-02 14:25:00-05'::timestamptz, 'DENUNCIADO', 'Patricia Salas Méndez CAL 20019')
)
INSERT INTO escritos (title, type, expediente_number, document_date, file_url, status, created_at, updated_at)
SELECT v.title, v.type, e.number, v.document_date,
       (ARRAY['https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf','https://www.adobe.com/content/dam/cc1/en/accessibility/products/acrobat/pdfs/acrobat-x-pdf-accessibility-best-practices.pdf','https://wwwimages2.adobe.com/content/dam/cc/en/accessibility/pdfs/accessing-pdf-sr.pdf'])[1 + ((row_number() OVER (ORDER BY e.id, v.title) - 1) % 3)::integer],
       v.status, v.created_at, v.created_at
FROM demo_escritos v
JOIN expedientes e ON e.name = v.expediente_name;

INSERT INTO escrito_denunciantes (escrito_id, denunciante_id)
SELECT es.id, p.id
FROM escritos es
JOIN expedientes e ON e.number = es.expediente_number AND e.name LIKE '[DEMO]%'
JOIN (VALUES
    ('[DEMO] Escrito de solicitud de acceso al expediente', 'Valeria Rojas Salazar'),
    ('[DEMO] Escrito de ampliación de denuncia', 'Martín Paredes León'),
    ('[DEMO] Escrito de entrega de información', 'Inversiones Valle Claro S.A.C.')
) AS v(escrito_title, person_name) ON v.escrito_title = es.title
JOIN denunciantes p ON p.name = v.person_name;

INSERT INTO escrito_denunciados (escrito_id, denunciado_id)
SELECT es.id, p.id
FROM escritos es
JOIN expedientes e ON e.number = es.expediente_number AND e.name LIKE '[DEMO]%'
JOIN (VALUES
    ('[DEMO] Escrito de descargos y anexos', 'Andrés Quiroz Vega CAL 20016'),
    ('[DEMO] Escrito de respuesta a requerimiento', 'Sergio Valdivia Ponce CAL 20018'),
    ('[DEMO] Escrito de observaciones a la resolución', 'Patricia Salas Méndez CAL 20019')
) AS v(escrito_title, person_name) ON v.escrito_title = es.title
JOIN denunciados p ON p.name = v.person_name;

COMMIT;

-- Comprobación rápida después de ejecutar:
-- SELECT count(*) FROM expedientes WHERE name LIKE '[DEMO]%';       -- 3
-- SELECT count(*) FROM documents WHERE name LIKE '[DEMO]%';         -- 45 with the default 15 substages
-- SELECT count(*) FROM escritos WHERE title LIKE '[DEMO]%';         -- 6
