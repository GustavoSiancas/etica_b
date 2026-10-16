# Expedientes

Cada expediente recibe un c?digo consecutivo ?nico (`1-2026`, `2-2026`, etc.), un nombre y un tipo de inicio. Al crearlo tambi?n se env?a `opening_date`, la fecha de apertura. Es independiente de `created_at`, que registra cu?ndo se cre? en el sistema. Puede ser una fecha pasada o futura.

## Opciones de inicio

`GET /tipos-inicio` devuelve los valores que el frontend puede seleccionar:

```json
[
  {"value":"DENUNCIA_DE_OFICIO","label":"DENUNCIA DE OFICIO"},
  {"value":"DENUNCIA_DE_PARTE","label":"DENUNCIA DE PARTE"},
  {"value":"COMUNICACIONES_DE_ENTIDADES_PUBLICAS","label":"COMUNICACIONES DE ENTIDADES PUBLICAS"},
  {"value":"COMUNICACIONES_DE_SANCION_IMPUESTA_POR_PJ_O_MP","label":"COMUNICACIONES DE SANCION IMPUESTA POR PJ O MP"},
  {"value":"APROBADO_POR_ASAMBLEA_GENERAL","label":"APROBADO POR ASAMBLEA GENERAL"}
]
```

## Estados sugeridos

`GET /expediente-statuses` devuelve el cat?logo inicial para poblar selectores del frontend:

```json
[
  {"value":"OPEN","label":"Abierto"},
  {"value":"IN_PROGRESS","label":"En proceso"},
  {"value":"CLOSED","label":"Cerrado"}
]
```

El PATCH de estado contin?a aceptando texto libre de hasta 30 caracteres; este cat?logo es una lista sugerida y no restringe otros valores.

## Crear expediente

Enviar el valor elegido. El cliente no envía el código consecutivo ni el estado:

```http
POST /expedientes
Content-Type: application/json

{"name":"Caso de ejemplo","opening_date":"2026-10-20","init_type":"DENUNCIA_DE_PARTE"}
```

La respuesta `201 Created` incluye el valor enum directamente:

```json
{
  "id": 2,
  "code": "2-2026",
  "number": "2-2026",
  "name": "Caso de ejemplo",
  "opening_date": "2026-10-20",
  "init_type": "DENUNCIA_DE_PARTE",
  "status": "OPEN",
  "created_at": "2026-10-15T17:00:00Z",
  "updated_at": "2026-10-15T17:00:00Z"
}
```

El nombre y `opening_date` son obligatorios. La fecha usa el formato `YYYY-MM-DD`; no se compara con la fecha actual, as? que admite fechas futuras. Un valor ausente o desconocido de `init_type` devuelve 400. PostgreSQL restringe la columna al enum. Al añadir un valor, ejecutar `ALTER TYPE expediente_init_type_enum ADD VALUE 'NUEVO_VALOR';` y actualizar la lista estática de opciones en `internal/services/expedientes.go`.

## Listar expedientes

`GET /expedientes?limit=20&offset=0`. limit predeterminado 20, mínimo 1 y máximo 100. offset predeterminado 0 y no negativo. Orden `created_at DESC, id DESC`. Los expedientes existentes anteriores a esta función pueden tener `init_type: null` porque su tipo de origen se desconoce.

```json
{"data":[],"total":0,"limit":20,"offset":0}
```

## Migración

Aplicar en orden `001_ethics.up.sql`, seed `001_document_catalogs.sql`, `002_expediente_codes.up.sql`, `003_expediente_init_types.up.sql`, `004_expediente_init_enum.up.sql`, `005_participant_membership_active.up.sql` y `006_expediente_name_opening_date.up.sql`. La migración 004 sustituye el catálogo intermedio por el enum y conserva los tipos previamente seleccionados. No usar AutoMigrate.

## Ejemplo PowerShell

```powershell
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/tipos-inicio'
Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/expedientes' -ContentType 'application/json' -Body '{"name":"Caso de ejemplo","opening_date":"2026-10-20","init_type":"DENUNCIA_DE_PARTE"}'
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/expedientes?limit=20&offset=0'
```


## Actualizar estado

`PATCH /expedientes/{id}/status` acepta cualquier estado de texto de hasta 30 caracteres no vac?os. Se eliminan espacios al inicio y final; no hay cat?logo fijo de estados.

```json
{"status":"EN INVESTIGACION"}
```

Devuelve `200` con el expediente actualizado, `400` si el ID o estado son inv?lidos y `404` si el expediente no existe.
