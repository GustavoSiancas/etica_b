-- 50 sample participants; all document numbers below are fictional test values.
-- Names are real company names/public historical figures; no expediente links.
-- Only complete previously seeded demo rows whose document is still NULL.
BEGIN;
LOCK TABLE denunciantes IN SHARE ROW EXCLUSIVE MODE;
WITH seed(name, document_type, document_number) AS (VALUES
    ('Banco de Crédito del Perú', 'RUC', '20990000001'),
    ('Interbank', 'RUC', '20990000002'),
    ('BBVA Perú', 'RUC', '20990000003'),
    ('Scotiabank Perú', 'RUC', '20990000004'),
    ('Alicorp', 'RUC', '20990000005'),
    ('Gloria', 'RUC', '20990000006'),
    ('Backus', 'RUC', '20990000007'),
    ('Ferreycorp', 'RUC', '20990000008'),
    ('Cementos Pacasmayo', 'RUC', '20990000009'),
    ('UNACEM', 'RUC', '20990000010'),
    ('Buenaventura', 'RUC', '20990000011'),
    ('Minsur', 'RUC', '20990000012'),
    ('Southern Copper Corporation', 'RUC', '20990000013'),
    ('Rímac Seguros', 'RUC', '20990000014'),
    ('Pacífico Seguros', 'RUC', '20990000015'),
    ('La Positiva Seguros', 'RUC', '20990000016'),
    ('InRetail Perú', 'RUC', '20990000017'),
    ('Supermercados Peruanos', 'RUC', '20990000018'),
    ('Saga Falabella', 'RUC', '20990000019'),
    ('Ripley Perú', 'RUC', '20990000020'),
    ('Promart', 'RUC', '20990000021'),
    ('Sodimac Perú', 'RUC', '20990000022'),
    ('Entel Perú', 'RUC', '20990000023'),
    ('Claro Perú', 'RUC', '20990000024'),
    ('LATAM Airlines Perú', 'RUC', '20990000025'),
    ('Miguel Grau Seminario', 'DNI', '99000001'),
    ('Francisco Bolognesi Cervantes', 'DNI', '99000002'),
    ('José Olaya Balandra', 'DNI', '99000003'),
    ('Andrés Avelino Cáceres', 'DNI', '99000004'),
    ('José Abelardo Quiñones Gonzales', 'DNI', '99000005'),
    ('María Parado de Bellido', 'DNI', '99000006'),
    ('Micaela Bastidas Puyucahua', 'DNI', '99000007'),
    ('José Gabriel Condorcanqui', 'DNI', '99000008'),
    ('Manuela Sáenz', 'DNI', '99000009'),
    ('José de San Martín', 'DNI', '99000010'),
    ('Simón Bolívar', 'DNI', '99000011'),
    ('César Vallejo', 'DNI', '99000012'),
    ('José María Arguedas', 'DNI', '99000013'),
    ('Ciro Alegría', 'DNI', '99000014'),
    ('Ricardo Palma', 'DNI', '99000015'),
    ('Abraham Valdelomar', 'DNI', '99000016'),
    ('Clorinda Matto de Turner', 'DNI', '99000017'),
    ('Mercedes Cabello de Carbonera', 'DNI', '99000018'),
    ('José Carlos Mariátegui', 'DNI', '99000019'),
    ('Jorge Basadre Grohmann', 'DNI', '99000020'),
    ('Julio C. Tello', 'CE', '009900001'),
    ('Daniel Alcides Carrión', 'CE', '009900002'),
    ('Pedro Paulet', 'CE', '009900003'),
    ('Chabuca Granda', 'CE', '009900004'),
    ('Yma Súmac', 'CE', '009900005')
), completed AS (
    UPDATE denunciantes d
    SET document_type=s.document_type, document_number=s.document_number, updated_at=NOW()
    FROM seed s
    WHERE d.is_demo=TRUE
      AND lower(btrim(d.name))=lower(s.name)
      AND d.document_type IS NULL AND d.document_number IS NULL
    RETURNING d.id
)
INSERT INTO denunciantes (name, document_type, document_number, is_demo)
SELECT s.name, s.document_type, s.document_number, TRUE
FROM seed s
WHERE NOT EXISTS (
    SELECT 1 FROM denunciantes d WHERE lower(btrim(d.name))=lower(s.name)
);
COMMIT;
