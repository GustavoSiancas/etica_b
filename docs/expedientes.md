# Expedientes

Cada expediente recibe un c?digo consecutivo ?nico (`1-2026`, `2-2026`, etc.), una descripcion y un tipo de inicio. Al crearlo tambi?n se env?a `opening_date`, la fecha de apertura. Es independiente de `created_at`, que registra cu?ndo se cre? en el sistema. Puede ser una fecha pasada o futura.

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

{"description":"Caso de ejemplo","opening_date":"2026-10-20","init_type":"DENUNCIA_DE_PARTE","denunciante_ids":[1,2],"denunciado_ids":[3]}
```

La respuesta `201 Created` incluye el valor enum directamente:

```json
{
  "id": 2,
  "code": "2-2026",
  "number": "2-2026",
  "description": "Caso de ejemplo",
  "opening_date": "2026-10-20",
  "init_type": "DENUNCIA_DE_PARTE",
  "status": "OPEN",
  "created_at": "2026-10-15T17:00:00Z",
  "updated_at": "2026-10-15T17:00:00Z"
}
```

La descripcion y `opening_date` son obligatorios. La fecha usa el formato `YYYY-MM-DD`; no se compara con la fecha actual, as? que admite fechas futuras. Un valor ausente o desconocido de `init_type` devuelve 400. PostgreSQL restringe la columna al enum. Al añadir un valor, ejecutar `ALTER TYPE expediente_init_type_enum ADD VALUE 'NUEVO_VALOR';` y actualizar la lista estática de opciones en `internal/services/expedientes.go`.

Los IDs de participantes se envian en `denunciante_ids` y `denunciado_ids`. Deben existir previamente por la carga inicial. Las listas admiten cero o mas IDs, sin duplicados ni valores no positivos. El servicio crea expediente y asociaciones en una sola transaccion; si falta un participante devuelve 404 y revierte toda la operacion. No existen POST separados para asociar participantes despues.

## Listar expedientes

`GET /expedientes?limit=20&offset=0`. limit predeterminado 20, mínimo 1 y máximo 100. offset predeterminado 0 y no negativo. Orden `created_at DESC, id DESC`. Los expedientes existentes anteriores a esta función pueden tener `init_type: null` porque su tipo de origen se desconoce.

```json
{"data":[],"total":0,"limit":20,"offset":0}
```

## Migración

El esquema completo esta en `migrations/001_initial.up.sql` y se aplica una sola vez a una base nueva. `migrations/001_initial.down.sql` revierte la instalacion eliminando los datos. No hay pasos incrementales ni actualizacion automatica de bases anteriores. No usar AutoMigrate.

## Ejemplo PowerShell

```powershell
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/tipos-inicio'
Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/expedientes' -ContentType 'application/json' -Body '{"description":"Caso de ejemplo","opening_date":"2026-10-20","init_type":"DENUNCIA_DE_PARTE","denunciante_ids":[1,2],"denunciado_ids":[3]}'
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/expedientes?limit=20&offset=0'
```


## Actualizar estado

`PATCH /expedientes/{id}/status` acepta cualquier estado de texto de hasta 30 caracteres no vac?os. Se eliminan espacios al inicio y final; no hay cat?logo fijo de estados.

```json
{"status":"EN INVESTIGACION"}
```

Devuelve `200` con el expediente actualizado, `400` si el ID o estado son inv?lidos y `404` si el expediente no existe.
