# Denunciantes y denunciados

Denunciantes y denunciados son registros referenciales con nombre. No se exige que el nombre sea único y se puede asociar el mismo registro con varios expedientes. No se envían datos personales adicionales.

## Crear y listar

`POST /denunciantes` y `POST /denunciados` reciben:

```json
{"name":"Nombre de la persona"}
```

Devuelven `201 Created` con `{ "id": 1, "name": "Nombre de la persona" }`. El nombre es obligatorio; se recortan espacios exteriores y se permiten hasta 255 caracteres.

`GET /denunciantes?limit=20&offset=0` y `GET /denunciados?limit=20&offset=0` devuelven:

```json
{"data":[{"id":1,"name":"Nombre de la persona"}],"total":1,"limit":20,"offset":0}
```

El orden es por ID ascendente; limit acepta 1–100 y offset debe ser no negativo.

## Asociar al expediente

`POST /expedientes/{expediente_id}/denunciantes`:

```json
{"denunciante_id":1}
```

`POST /expedientes/{expediente_id}/denunciados`:

```json
{"denunciado_id":2}
```

Cada endpoint devuelve `201 Created` con el ID de la relación, expediente, persona y `is_active: true`. La relación duplicada y ya activa devuelve `409`; expediente inexistente, `404`; persona inexistente, `404`. Las claves foráneas y restricciones únicas de las migraciones protegen la relación.

Las asociaciones se conservan en `expediente_denunciantes` y `expediente_denunciados` con `is_active`. Para retirar sin borrar la relación, llamar `DELETE /expedientes/{expediente_id}/denunciantes/{denunciante_id}` o `DELETE /expedientes/{expediente_id}/denunciados/{denunciado_id}`. La respuesta es `204`. La asociación pasa a inactiva. Un retiro repetido devuelve `404`.

Al asociar de nuevo una persona previamente retirada, se reactiva la misma fila, conserva su ID y crea un nuevo evento `ADDED`. No se crea una relación duplicada. Si ya está activa, devuelve `409`.

Cada activación y retiro y su evento `ADDED`/`REMOVED` del historial se guardan juntos en una transacción. El historial contiene tipo, ID y snapshot del nombre para poder reconstruir el cambio. No se crean estados o fechas de notificación y no se borran físicamente las asociaciones.

## Ejemplo PowerShell

```powershell
$p = Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/denunciantes' -ContentType 'application/json' -Body '{"name":"María Pérez"}'
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/denunciantes'
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/expedientes/1/denunciantes" -ContentType 'application/json' -Body "{`"denunciante_id`":$($p.id)}"
Invoke-RestMethod -Method Delete -Uri "http://localhost:8080/expedientes/1/denunciantes/$($p.id)"
```

## Listar participantes activos del expediente

`GET /expedientes/{id}/denunciantes` y `GET /expedientes/{id}/denunciados` devuelven solo asociaciones activas del expediente, ordenadas por ID de asociaci?n ascendente, sin paginaci?n.

```json
[
  {"id":5,"expediente_id":1,"participant":{"id":2,"name":"Ana"},"is_active":true}
]
```

`id` es el ID de la asociaci?n (para referencias en documentos); `participant.id` es el ID de la persona (para retirarla del expediente). Devuelven `200` y `[]` si el expediente existe sin participantes activos, `400` para un ID inv?lido y `404` si el expediente no existe.
