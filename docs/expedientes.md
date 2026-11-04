# Expedientes

Cada expediente recibe un `number` consecutivo anual (`1-2026`, `2-2026`, etc.) generado por PostgreSQL según el año de `opening_date`. No existe `code`. La fecha de apertura indicada por el usuario es independiente de `created_at`; acepta fechas pasadas o futuras.

Por ejemplo, aperturas en 2025 reciben `1-2025`, `2-2025`, etc.; aperturas en 2026 usan otro contador: `1-2026`, `2-2026`, etc. El año de registro en el sistema no determina el número. La migración 003 actualiza automáticamente el trigger al arrancar y conserva los números ya asignados.

## Crear y listar

`GET /tipos-inicio` devuelve los tipos permitidos: `DENUNCIA_DE_OFICIO`, `DENUNCIA_DE_PARTE`, `COMUNICACIONES_DE_ENTIDADES_PUBLICAS`, `COMUNICACIONES_DE_SANCION_IMPUESTA_POR_PJ_O_MP` y `APROBADO_POR_ASAMBLEA_GENERAL`.

```http
POST /expedientes
Content-Type: application/json

{"description":"Caso de ejemplo","opening_date":"2026-10-30","init_type":"DENUNCIA_DE_PARTE","denunciante_ids":[1,2],"denunciado_ids":[3]}
```

`opening_date` e `init_type` son obligatorios. La descripción es opcional y se guarda como texto vacío si se omite. Las listas de participantes son opcionales; los IDs deben existir, ser positivos y no repetirse. La creación guarda expediente y asociaciones en una transacción. Los participantes se cargan inicialmente y no existen endpoints para cambiar las asociaciones después.

El estado y la ubicación iniciales siempre son `ABIERTO` y `DIRECCION DE ETICA`. No se aceptan esos campos en la creación.

Respuesta 201:

```json
{
  "id": 2,
  "number": "2-2026",
  "description": "Caso de ejemplo",
  "opening_date": "2026-10-30",
  "init_type": "DENUNCIA_DE_PARTE",
  "status": "ABIERTO",
  "location": "DIRECCION DE ETICA",
  "created_at": "2026-10-30T15:00:00Z",
  "updated_at": "2026-10-30T15:00:00Z"
}
```

`GET /expedientes?limit=20&offset=0` devuelve `{data,total,limit,offset}` ordenado por creación descendente e ID. `limit` admite 1–100 y `offset` desde cero. Incluye `status` y `location` para cada expediente.

Filtros opcionales, combinables con la paginación:

- `status=ABIERTO` o `status=CERRADO`. Si se omite, lista ambos estados.
- `location=DIRECCION%20DE%20ETICA`, `location=CONSEJO%20DE%20ETICA` o `location=TRIBUNAL%20DE%20HONOR`. Si se omite, lista todas las ubicaciones.
- `from=2026-10-01` y `to=2026-10-31`: filtran la fecha de apertura (`opening_date`), incluyendo ambos días. Se permite enviar solo un límite. Si se envían ambos, `from` debe ser menor o igual que `to`. Fechas inválidas, formatos distintos de YYYY-MM-DD o parámetros repetidos devuelven 400. Con un límite de fecha se excluyen expedientes sin fecha de apertura.

```http
GET /expedientes?from=2026-10-01&to=2026-10-31&status=ABIERTO&limit=20&offset=0
```

El mismo rango está disponible en los [expedientes por denunciante y denunciado](expedientes-por-participante.md). Se combina con los demás filtros antes de contar y paginar.

Ejemplo: `GET /expedientes?status=ABIERTO&location=CONSEJO%20DE%20ETICA&limit=20&offset=0`. `total` cuenta todos los resultados filtrados antes de aplicar `limit` y `offset`. Valores desconocidos o filtros repetidos devuelven 400.

## Estado

`GET /expediente-statuses` devuelve exclusivamente:

```json
[
  {"value":"ABIERTO","label":"Abierto"},
  {"value":"CERRADO","label":"Cerrado"}
]
```

`PATCH /expedientes/{id}/status` cierra o reabre explícitamente:

```json
{"status":"CERRADO"}
```

Para reabrir, enviar `{"status":"ABIERTO"}`. El cambio conserva la ubicación. Estados anteriores como OPEN, CLOSED, IN_PROGRESS o cualquier texto libre devuelven 400.

## Ubicación

`GET /expediente-locations` devuelve los valores permitidos:

- `DIRECCION DE ETICA`
- `CONSEJO DE ETICA`
- `TRIBUNAL DE HONOR`

`PATCH /expedientes/{id}/location`:

```json
{"location":"CONSEJO DE ETICA"}
```

La ubicación es independiente del estado y puede cambiarse con el expediente ABIERTO o CERRADO. Cambiarla conserva el estado y no desbloquea su contenido. Ambos PATCH devuelven el expediente completo actualizado (200); datos inválidos producen 400 y un expediente inexistente 404.

## Cierre y bloqueo de información

Un expediente CERRADO se puede consultar, pero devuelve 409 al intentar:

- Crear, editar, mover, ordenar o eliminar secciones y subsecciones.
- Crear documentos dentro de sus secciones, vincularlos o desvincularlos.
- Editar o eliminar documentos vinculados, incluso desde endpoints globales.
- Solicitar firmas de carga o confirmar enlaces de sus documentos.
- Agregar asociaciones de participantes desde servicios internos.

Los documentos compartidos se bloquean para edición si cualquiera de sus expedientes está cerrado. Las lecturas siguen disponibles. Un documento independiente sin vínculos puede crearse o editarse porque todavía no pertenece a un expediente; vincularlo a uno cerrado está bloqueado.

Reabrir mediante el endpoint de estado habilita nuevamente las operaciones. Si otro propietario de un documento compartido sigue cerrado, ese documento continúa bloqueado. Las mutaciones se coordinan con locks de base de datos para evitar escrituras simultáneas al cierre. Una firma de carga emitida antes de cerrar puede seguir vigente en R2, pero su confirmación no modifica el documento mientras el expediente esté cerrado.

## Esquema

`001_initial.up.sql` es la única migración consolidada para bases nuevas. Incluye restricciones SQL para los dos estados y las tres ubicaciones. La migraci?n 002 se aplica al arrancar para a?adir ubicaci?n y convertir los estados anteriores en bases existentes; no usar AutoMigrate. Las reglas de cierre se aplican en los servicios de la aplicación; las herramientas de reset y limpieza de desarrollo siguen siendo operaciones administrativas de prueba.

## Actualizar descripcion

`PATCH /expedientes/{id}/description`

```json
{"description":"Nueva descripcion del expediente"}
```

Devuelve 200 con el expediente actualizado y `updated_at` en UTC. El campo es obligatorio, debe ser texto y se recortan los espacios externos. Enviar `""` borra la descripcion; omitirlo, enviar `null`, un ID invalido o campos desconocidos devuelve 400. Devuelve 404 si el expediente no existe y 409 si esta cerrado: primero debe reabrirse.
