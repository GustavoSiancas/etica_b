-- DATOS COMPLETAMENTE FICTICIOS PARA DESARROLLO Y PRUEBAS.
-- Ejecutar después de migrations/001_initial.up.sql en una base de pruebas vacía.
-- Solo elimina y reemplaza filas con prefijo [DEMO]; puede ejecutarse repetidamente.
-- Los adjuntos PDF de demostración se guardan como data URI en file_url;
-- no son archivos de R2 y no deben usarse en producción.

BEGIN;

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
DELETE FROM denunciantes WHERE name LIKE '[DEMO]%';
DELETE FROM denunciados WHERE name LIKE '[DEMO]%';
DELETE FROM expedientes WHERE name LIKE '[DEMO]%';

INSERT INTO expedientes (name, opening_date, status, init_type, created_at, updated_at) VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', '2025-02-14', 'IN_PROGRESS', 'DENUNCIA_DE_PARTE', '2025-02-14 09:15:00-05', '2025-03-20 16:30:00-05'),
    ('[DEMO] Uso de recursos institucionales', '2025-06-03', 'OPEN', 'DENUNCIA_DE_OFICIO', '2025-06-03 10:00:00-05', '2025-06-03 10:00:00-05'),
    ('[DEMO] Incumplimiento de deberes funcionales', '2026-01-19', 'CLOSED', 'COMUNICACIONES_DE_ENTIDADES_PUBLICAS', '2026-01-19 08:45:00-05', '2026-05-22 17:20:00-05');

INSERT INTO denunciantes (name, created_at, updated_at) VALUES
    ('[DEMO] Valeria Rojas Salazar', '2025-02-14 09:10:00-05', '2025-02-14 09:10:00-05'),
    ('[DEMO] Martín Paredes León', '2025-02-14 09:12:00-05', '2025-02-14 09:12:00-05'),
    ('[DEMO] Rosa Huamán Torres', '2025-06-03 09:55:00-05', '2025-06-03 09:55:00-05'),
    ('[DEMO] Diego Cárdenas Ruiz', '2026-01-19 08:40:00-05', '2026-01-19 08:40:00-05');

INSERT INTO denunciados (name, created_at, updated_at) VALUES
    ('[DEMO] Andrés Quiroz Vega', '2025-02-14 09:11:00-05', '2025-02-14 09:11:00-05'),
    ('[DEMO] Lucía Benavides Cruz', '2025-02-14 09:13:00-05', '2025-02-14 09:13:00-05'),
    ('[DEMO] Sergio Valdivia Ponce', '2025-06-03 09:56:00-05', '2025-06-03 09:56:00-05'),
    ('[DEMO] Patricia Salas Méndez', '2026-01-19 08:41:00-05', '2026-01-19 08:41:00-05');

INSERT INTO expediente_denunciantes (expediente_id, denunciante_id, is_active)
SELECT e.id, p.id, TRUE
FROM (VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Valeria Rojas Salazar'),
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Martín Paredes León'),
    ('[DEMO] Uso de recursos institucionales', '[DEMO] Rosa Huamán Torres'),
    ('[DEMO] Incumplimiento de deberes funcionales', '[DEMO] Diego Cárdenas Ruiz')
) AS v(expediente_name, person_name)
JOIN expedientes e ON e.name = v.expediente_name
JOIN denunciantes p ON p.name = v.person_name;

INSERT INTO expediente_denunciados (expediente_id, denunciado_id, is_active)
SELECT e.id, p.id, TRUE
FROM (VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Andrés Quiroz Vega'),
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Lucía Benavides Cruz'),
    ('[DEMO] Uso de recursos institucionales', '[DEMO] Sergio Valdivia Ponce'),
    ('[DEMO] Incumplimiento de deberes funcionales', '[DEMO] Patricia Salas Méndez')
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

-- PDF sintético de una página, contenido íntegramente en este SQL.
-- Todos los documentos de expediente apuntan a este adjunto de prueba.
WITH demo_docs(expediente_name, substage_code, title, document_date, status, created_at) AS (VALUES
    ('[DEMO] Compra de suministros y conflicto de interés', 'CALIFICACION', '[DEMO] Denuncia por presunto conflicto de interés', '2025-02-14'::date, 'APPROVED', '2025-02-14 09:30:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'PROYECCION', '[DEMO] Proyecto de calificación de denuncia', '2025-02-20'::date, 'APPROVED', '2025-02-20 14:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'REVISION_FIRMA', '[DEMO] Resolución de inicio de investigación', '2025-02-24'::date, 'APPROVED', '2025-02-24 11:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'NOTIFICACION_RESOLUCION', '[DEMO] Notificación de resolución de inicio', '2025-02-26'::date, 'APPROVED', '2025-02-26 10:20:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'CARGOS_NOTIFICACION', '[DEMO] Cargo de notificación de inicio', '2025-02-27'::date, 'APPROVED', '2025-02-27 16:15:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'DESCARGOS_ESCRITOS', '[DEMO] Descargos de Andrés Quiroz', '2025-03-05'::date, 'APPROVED', '2025-03-05 12:30:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'FORMALIZACION_ESCRITOS', '[DEMO] Formalización de descargos', '2025-03-07'::date, 'APPROVED', '2025-03-07 15:45:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'PROGRAMACION', '[DEMO] Programación de audiencia', '2025-03-10'::date, 'APPROVED', '2025-03-10 09:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'NOTIFICACION_AUDIENCIA', '[DEMO] Notificación de audiencia', '2025-03-11'::date, 'APPROVED', '2025-03-11 10:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'CARGOS_AUDIENCIA', '[DEMO] Cargos de notificación de audiencia', '2025-03-12'::date, 'APPROVED', '2025-03-12 16:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'ACTA_AUDIENCIA', '[DEMO] Acta de audiencia', '2025-03-17'::date, 'APPROVED', '2025-03-17 12:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'FICHA_VOTACION', '[DEMO] Ficha de votación', '2025-03-19'::date, 'APPROVED', '2025-03-19 11:30:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'RESOLUCION', '[DEMO] Resolución final', '2025-03-20'::date, 'APPROVED', '2025-03-20 15:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'NOTIFICACION_FINAL', '[DEMO] Notificación de resolución final', '2025-03-21'::date, 'PENDING', '2025-03-21 09:00:00-05'::timestamptz),
    ('[DEMO] Compra de suministros y conflicto de interés', 'CARGOS_FINAL', '[DEMO] Cargo de notificación final', '2025-03-24'::date, 'DRAFT', '2025-03-24 10:00:00-05'::timestamptz),
    ('[DEMO] Uso de recursos institucionales', 'CALIFICACION', '[DEMO] Denuncia sobre uso de vehículo institucional', '2025-06-03'::date, 'PENDING', '2025-06-03 10:15:00-05'::timestamptz),
    ('[DEMO] Uso de recursos institucionales', 'PROYECCION', '[DEMO] Proyecto de calificación sobre recursos', '2025-06-09'::date, 'DRAFT', '2025-06-09 13:00:00-05'::timestamptz),
    ('[DEMO] Uso de recursos institucionales', 'DESCARGOS_ESCRITOS', '[DEMO] Descargos de Sergio Valdivia', '2025-06-18'::date, 'DRAFT', '2025-06-18 15:10:00-05'::timestamptz),
    ('[DEMO] Uso de recursos institucionales', 'PROGRAMACION', '[DEMO] Programación de entrevista', '2025-06-25'::date, 'DRAFT', '2025-06-25 09:40:00-05'::timestamptz),
    ('[DEMO] Uso de recursos institucionales', 'ACTA_AUDIENCIA', '[DEMO] Acta de entrevista', '2025-07-02'::date, 'DRAFT', '2025-07-02 12:20:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'CALIFICACION', '[DEMO] Comunicación de entidad pública', '2026-01-19'::date, 'APPROVED', '2026-01-19 09:10:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'PROYECCION', '[DEMO] Proyecto de calificación de comunicación', '2026-01-23'::date, 'APPROVED', '2026-01-23 14:30:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'REVISION_FIRMA', '[DEMO] Resolución de apertura de procedimiento', '2026-01-27'::date, 'APPROVED', '2026-01-27 11:00:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'DESCARGOS_ESCRITOS', '[DEMO] Descargos de Patricia Salas', '2026-02-10'::date, 'APPROVED', '2026-02-10 16:00:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'PROGRAMACION', '[DEMO] Programación de audiencia final', '2026-03-03'::date, 'APPROVED', '2026-03-03 10:15:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'ACTA_AUDIENCIA', '[DEMO] Acta de audiencia final', '2026-03-12'::date, 'APPROVED', '2026-03-12 13:00:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'FICHA_VOTACION', '[DEMO] Ficha de votación del caso', '2026-05-18'::date, 'APPROVED', '2026-05-18 15:20:00-05'::timestamptz),
    ('[DEMO] Incumplimiento de deberes funcionales', 'RESOLUCION', '[DEMO] Resolución final del caso', '2026-05-22'::date, 'APPROVED', '2026-05-22 16:30:00-05'::timestamptz)
)
INSERT INTO documents (expediente_id, name, file_url, document_date, substage_id, status, created_at, updated_at)
SELECT e.id, v.title,
       'data:application/pdf;base64,JVBERi0xLjQKMSAwIG9iago8PCAvVHlwZSAvQ2F0YWxvZyAvUGFnZXMgMiAwIFIgPj4KZW5kb2JqCjIgMCBvYmoKPDwgL1R5cGUgL1BhZ2VzIC9LaWRzIFszIDAgUl0gL0NvdW50IDEgPj4KZW5kb2JqCjMgMCBvYmoKPDwgL1R5cGUgL1BhZ2UgL1BhcmVudCAyIDAgUiAvTWVkaWFCb3ggWzAgMCA2MTIgNzkyXSAvUmVzb3VyY2VzIDw8IC9Gb250IDw8IC9GMSA0IDAgUiA+PiA+PiAvQ29udGVudHMgNSAwIFIgPj4KZW5kb2JqCjQgMCBvYmoKPDwgL1R5cGUgL0ZvbnQgL1N1YnR5cGUgL1R5cGUxIC9CYXNlRm9udCAvSGVsdmV0aWNhID4+CmVuZG9iago1IDAgb2JqCjw8IC9MZW5ndGggMTI1ID4+CnN0cmVhbQpCVCAvRjEgMTYgVGYgNzIgNzQwIFRkIChET0NVTUVOVE8gRklDVElDSU8gREUgUFJVRUJBKSBUaiAwIC0yOCBUZCAvRjEgMTEgVGYgKENvbnRlbmlkbyBmaWN0aWNpbyBnZW5lcmFkbyBwYXJhIHBydWViYXMuKSBUaiBFVAplbmRzdHJlYW0KZW5kb2JqCnhyZWYKMCA2CjAwMDAwMDAwMDAgNjU1MzUgZiAKMDAwMDAwMDAwOSAwMDAwMCBuIAowMDAwMDAwMDU4IDAwMDAwIG4gCjAwMDAwMDAxMTUgMDAwMDAgbiAKMDAwMDAwMDI0MSAwMDAwMCBuIAowMDAwMDAwMzExIDAwMDAwIG4gCnRyYWlsZXIKPDwgL1NpemUgNiAvUm9vdCAxIDAgUiA+PgpzdGFydHhyZWYKNDg3CiUlRU9G',
       v.document_date, ss.id, v.status, v.created_at, v.created_at
FROM demo_docs v
JOIN expedientes e ON e.name = v.expediente_name
JOIN document_substages ss ON ss.code = v.substage_code;

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
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Escrito de solicitud de acceso al expediente', 'Solicitud de acceso', '2025-02-18'::date, 'ATENDIDA', '2025-02-18 10:05:00-05'::timestamptz, 'DENUNCIANTE', '[DEMO] Valeria Rojas Salazar'),
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Escrito de descargos y anexos', 'Descargos', '2025-03-05'::date, 'ATENDIDA', '2025-03-05 12:30:00-05'::timestamptz, 'DENUNCIADO', '[DEMO] Andrés Quiroz Vega'),
    ('[DEMO] Compra de suministros y conflicto de interés', '[DEMO] Escrito de ampliación de denuncia', 'Ampliación de denuncia', '2025-03-08'::date, 'NUEVA', '2025-03-08 09:40:00-05'::timestamptz, 'DENUNCIANTE', '[DEMO] Martín Paredes León'),
    ('[DEMO] Uso de recursos institucionales', '[DEMO] Escrito de entrega de información', 'Entrega de información', '2025-06-12'::date, 'ATENDIDA', '2025-06-12 11:15:00-05'::timestamptz, 'DENUNCIANTE', '[DEMO] Rosa Huamán Torres'),
    ('[DEMO] Uso de recursos institucionales', '[DEMO] Escrito de respuesta a requerimiento', 'Respuesta a requerimiento', '2025-06-20'::date, 'NUEVA', '2025-06-20 16:10:00-05'::timestamptz, 'DENUNCIADO', '[DEMO] Sergio Valdivia Ponce'),
    ('[DEMO] Incumplimiento de deberes funcionales', '[DEMO] Escrito de observaciones a la resolución', 'Observaciones', '2026-04-02'::date, 'ATENDIDA', '2026-04-02 14:25:00-05'::timestamptz, 'DENUNCIADO', '[DEMO] Patricia Salas Méndez')
)
INSERT INTO escritos (title, type, expediente_number, document_date, file_url, status, created_at, updated_at)
SELECT v.title, v.type, e.number, v.document_date,
       'data:application/pdf;base64,JVBERi0xLjQKMSAwIG9iago8PCAvVHlwZSAvQ2F0YWxvZyAvUGFnZXMgMiAwIFIgPj4KZW5kb2JqCjIgMCBvYmoKPDwgL1R5cGUgL1BhZ2VzIC9LaWRzIFszIDAgUl0gL0NvdW50IDEgPj4KZW5kb2JqCjMgMCBvYmoKPDwgL1R5cGUgL1BhZ2UgL1BhcmVudCAyIDAgUiAvTWVkaWFCb3ggWzAgMCA2MTIgNzkyXSAvUmVzb3VyY2VzIDw8IC9Gb250IDw8IC9GMSA0IDAgUiA+PiA+PiAvQ29udGVudHMgNSAwIFIgPj4KZW5kb2JqCjQgMCBvYmoKPDwgL1R5cGUgL0ZvbnQgL1N1YnR5cGUgL1R5cGUxIC9CYXNlRm9udCAvSGVsdmV0aWNhID4+CmVuZG9iago1IDAgb2JqCjw8IC9MZW5ndGggMTIzID4+CnN0cmVhbQpCVCAvRjEgMTYgVGYgNzIgNzQwIFRkIChFU0NSSVRPIEZJQ1RJQ0lPIERFIFBSVUVCQSkgVGogMCAtMjggVGQgL0YxIDExIFRmIChDb250ZW5pZG8gZmljdGljaW8gZ2VuZXJhZG8gcGFyYSBwcnVlYmFzLikgVGogRVQKZW5kc3RyZWFtCmVuZG9iagp4cmVmCjAgNgowMDAwMDAwMDAwIDY1NTM1IGYgCjAwMDAwMDAwMDkgMDAwMDAgbiAKMDAwMDAwMDA1OCAwMDAwMCBuIAowMDAwMDAwMTE1IDAwMDAwIG4gCjAwMDAwMDAyNDEgMDAwMDAgbiAKMDAwMDAwMDMxMSAwMDAwMCBuIAp0cmFpbGVyCjw8IC9TaXplIDYgL1Jvb3QgMSAwIFIgPj4Kc3RhcnR4cmVmCjQ4NQolJUVPRg==',
       v.status, v.created_at, v.created_at
FROM demo_escritos v
JOIN expedientes e ON e.name = v.expediente_name;

INSERT INTO escrito_denunciantes (escrito_id, denunciante_id)
SELECT es.id, p.id
FROM escritos es
JOIN expedientes e ON e.number = es.expediente_number AND e.name LIKE '[DEMO]%'
JOIN (VALUES
    ('[DEMO] Escrito de solicitud de acceso al expediente', '[DEMO] Valeria Rojas Salazar'),
    ('[DEMO] Escrito de ampliación de denuncia', '[DEMO] Martín Paredes León'),
    ('[DEMO] Escrito de entrega de información', '[DEMO] Rosa Huamán Torres')
) AS v(escrito_title, person_name) ON v.escrito_title = es.title
JOIN denunciantes p ON p.name = v.person_name;

INSERT INTO escrito_denunciados (escrito_id, denunciado_id)
SELECT es.id, p.id
FROM escritos es
JOIN expedientes e ON e.number = es.expediente_number AND e.name LIKE '[DEMO]%'
JOIN (VALUES
    ('[DEMO] Escrito de descargos y anexos', '[DEMO] Andrés Quiroz Vega'),
    ('[DEMO] Escrito de respuesta a requerimiento', '[DEMO] Sergio Valdivia Ponce'),
    ('[DEMO] Escrito de observaciones a la resolución', '[DEMO] Patricia Salas Méndez')
) AS v(escrito_title, person_name) ON v.escrito_title = es.title
JOIN denunciados p ON p.name = v.person_name;

COMMIT;

-- Comprobación rápida después de ejecutar:
-- SELECT count(*) FROM expedientes WHERE name LIKE '[DEMO]%';       -- 3
-- SELECT count(*) FROM documents WHERE name LIKE '[DEMO]%';         -- 28
-- SELECT count(*) FROM escritos WHERE title LIKE '[DEMO]%';         -- 6
