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

La carpeta principal creada automaticamente comienza con `ABIERTO` y `DIRECCION DE ETICA`. El expediente comienza en `ABIERTO`; no admite `status` ni `location` en su creacion.

Respuesta 201:

```json
{
  "id": 2,
  "number": "2-2026",
  "description": "Caso de ejemplo",
  "opening_date": "2026-10-30",
  "init_type": "DENUNCIA_DE_PARTE",
  "created_at": "2026-10-30T15:00:00Z",
  "updated_at": "2026-10-30T15:00:00Z"
}
```

`GET /expedientes?limit=20&offset=0` devuelve `{data,total,limit,offset}` ordenado por creación descendente e ID. `limit` admite 1–100 y `offset` desde cero. El expediente devuelve su propio `status` y `closing_date`; las carpetas conservan su estado y ubicacion.

Filtros opcionales, combinables con la paginación:

- `status=ABIERTO` o `status=CERRADO`. Si se omite, lista ambos estados.
- `location=DIRECCION%20DE%20ETICA`, `location=CONSEJO%20DE%20ETICA` o `location=TRIBUNAL%20DE%20HONOR`. Si se omite, lista todas las ubicaciones.
- `from=2026-10-01` y `to=2026-10-31`: filtran la fecha de apertura (`opening_date`), incluyendo ambos días. Se permite enviar solo un límite. Si se envían ambos, `from` debe ser menor o igual que `to`. Fechas inválidas, formatos distintos de YYYY-MM-DD o parámetros repetidos devuelven 400. Con un límite de fecha se excluyen expedientes sin fecha de apertura.

```http
GET /expedientes?from=2026-10-01&to=2026-10-31&status=ABIERTO&limit=20&offset=0
```

El mismo rango está disponible en los [expedientes por denunciante y denunciado](expedientes-por-participante.md). Se combina con los demás filtros antes de contar y paginar.

Ejemplo: `GET /expedientes?status=ABIERTO&location=CONSEJO%20DE%20ETICA&limit=20&offset=0`. `total` cuenta todos los resultados filtrados antes de aplicar `limit` y `offset`. Valores desconocidos o filtros repetidos devuelven 400.

## Estado y ubicacion por carpeta

La migracion 008 retiro `status` y `location` del expediente; la migracion 009 agrega nuevamente `status` y agrega `closing_date` a expedientes y carpetas. La migracion `008_folder_state_location.up.sql` copia ambos valores historicos a la carpeta llamada `EXPEDIENTE PRINCIPAL` y retira las columnas del expediente. Las demas carpetas existentes comienzan en `ABIERTO` y `DIRECCION DE ETICA`. Los nuevos expedientes crean su carpeta principal con esos mismos valores por defecto.

- `GET /folder-statuses`: `ABIERTO`, `CERRADO`.
- `GET /folder-locations`: `DIRECCION DE ETICA`, `CONSEJO DE ETICA`, `TRIBUNAL DE HONOR`.
- `PATCH /expedientes/{expedienteID}/folders/{folderID}/status`: `{"status":"CERRADO"}`; enviar `ABIERTO` para reabrir.
- `PATCH /expedientes/{expedienteID}/folders/{folderID}/location`: `{"location":"CONSEJO DE ETICA"}`.

Ambos PATCH devuelven 200 con la carpeta actualizada, 400 para valores o IDs invalidos y 404 para expediente inexistente o carpeta ajena/inexistente. Cambiar ubicacion conserva el estado y esta permitido aunque la carpeta este cerrada. La ruta de ubicacion del expediente fue retirada.

Una carpeta cerrada bloquea con 409 cambios de sus secciones y vinculos. Las otras carpetas siguen siendo independientes. Los documentos compartidos se protegen contra edicion, eliminacion, firma y confirmacion de carga si cualquiera de sus carpetas esta cerrada. Las consultas permanecen disponibles. Cerrar una carpeta no bloquea los datos generales del expediente, sus participantes ni la creacion de carpetas adicionales.

Los filtros `status` y `location` de `GET /expedientes` y de las listas por participante buscan coincidencias en una misma carpeta; devuelven cada expediente una sola vez. `from` y `to` siguen filtrando la fecha de apertura del expediente.

## Actualizar descripcion

`PATCH /expedientes/{id}/description`

```json
{"description":"Nueva descripcion del expediente"}
```

Devuelve 200 con el expediente actualizado y `updated_at` en UTC. El campo es obligatorio, debe ser texto y se recortan los espacios externos. Enviar `""` borra la descripcion; omitirlo, enviar `null`, un ID invalido o campos desconocidos devuelve 400. Devuelve 404 si el expediente no existe. El estado de las carpetas no bloquea esta operacion.


### Cierre y reapertura

`PATCH /expedientes/{id}/status` acepta `{"status":"CERRADO"}` o `{"status":"ABIERTO"}` y devuelve el expediente actualizado. Cerrar registra la fecha actual de Peru en el expediente (`YYYY-MM-DD`) y cierra todas sus carpetas abiertas con el instante actual (`closing_date` en UTC). Las carpetas ya cerradas conservan su fecha y `updated_at`. Repetir un cierre no cambia su fecha.

Cerrar una carpeta registra su propia fecha de cierre. Si ya no quedan carpetas abiertas, tambien se cierra el expediente. Reabrir una carpeta limpia su fecha de cierre y reabre el expediente, limpiando la fecha de cierre de este. Reabrir directamente el expediente limpia solamente su fecha de cierre y conserva el estado y las fechas de todas sus carpetas. Las fechas de apertura permanecen intactas.

Crear una carpeta abierta reabre el expediente si estaba cerrado. Crear una carpeta cerrada asigna su fecha de cierre actual. Los cambios se realizan en transacciones y se serializan por expediente.


### Historial interno

La migracion `010_expediente_history.up.sql` crea `expediente_history`. Cada cambio conserva el ID del expediente, el ID y nombre de la carpeta cuando corresponde, el evento, el estado, las fechas de apertura y cierre y `changed_at` (instante del cambio). Las fechas calendario del expediente se interpretan a medianoche de Peru; la API del historial devuelve timestamps UTC.

`active_folders` es un JSONB interno con los IDs y nombres de todas las carpetas abiertas despues del cambio, por ejemplo `[{"id":12,"name":"EXPEDIENTE PRINCIPAL"}]`. Una lista vacia indica que no habia carpetas abiertas. Cada registro conserva su propia copia para auditoria; este campo no se expone en ninguna respuesta del historial.

El servicio registra automaticamente creaciones, aperturas, cierres, cambios de descripcion del expediente y cambios de ubicacion de carpetas en la misma transaccion que la operacion. Los cierres en cascada registran cada carpeta efectivamente cerrada. Repetir una apertura o cierre sin cambiar el estado no duplica eventos. Las carpetas que ya estaban cerradas conservan sus fechas y no reciben un nuevo evento de cierre. No hay endpoints de escritura del historial.

`GET /expedientes/{id}/history?limit=20&offset=0` devuelve `{data,total,limit,offset}`, ordenado por `changed_at DESC, id DESC`. Cada entrada contiene `id`, `expediente_id`, `folder_id`, `folder_name`, `event`, `status`, `location`, `opening_date`, `closing_date` y `changed_at`. Los campos de carpeta son nulos para eventos del expediente. `limit` permite 1 a 100; `offset` debe ser no negativo. Expediente inexistente devuelve 404; parametros invalidos, 400. Un expediente sin eventos devuelve una lista vacia.

El registro comienza al instalar esta version: no reconstruye aperturas o cierres pasados ni registra escrituras SQL externas al servicio. Los cambios del contenido de documentos y secciones no forman parte de este historial de estados.


### Historial de carpetas y contenido para auditoria

La migracion `011_folder_history_details.up.sql` amplia la misma tabla historica con `location` y `content_snapshot` (JSONB privado). Cada evento de apertura, cierre o cambio de ubicacion de una carpeta conserva el estado y la ubicacion de ese momento, sus fechas y una copia del contenido. Esto tambien se aplica a los cierres en cascada desde el expediente.

`GET /expedientes/{id}/folders/{folderID}/history?limit=20&offset=0` devuelve el historial de esa carpeta con el mismo formato paginado del historial del expediente. Incluye `location`, independiente de `status`: `DIRECCION DE ETICA`, `CONSEJO DE ETICA` o `TRIBUNAL DE HONOR`. Valida los IDs y la pertenencia de la carpeta al expediente; carpeta ajena o inexistente devuelve 404. No existen endpoints de escritura del historial.

`content_snapshot` contiene `schema_version: 1` y un arreglo `sections`. Cada seccion o subseccion guarda `id`, `name`, `parent_id`, `type`, `document_date`, `sort_order`, `color_hex`, `created_at`, `updated_at` y un arreglo `documents`. `parent_id` permite reconstruir toda la jerarquia, incluso subsecciones de varios niveles. Cada documento conserva su ID, nombre, fecha del documento, creacion y actualizacion. Una carpeta sin contenido guarda `sections: []`; una seccion sin documentos guarda `documents: []`. Solo se incluyen elementos de la carpeta del evento.

Los JSON `active_folders` y `content_snapshot` quedan exclusivamente para auditoria y no aparecen en ninguno de los GET de historial. Las copias historicas no cambian al renombrar documentos o modificar secciones posteriormente. La ubicacion y el contenido de eventos anteriores a la migracion 011 permanecen nulos, porque no se puede reconstruir su valor pasado a partir del estado actual. El JSON es una copia al momento del evento de estado o ubicacion, no un evento independiente por cada edicion del contenido.


### Consultar antes de cerrar una carpeta

`GET /expedientes/{expedienteID}/folders/{folderID}/status/closed` permite decidir con una sola llamada si mostrar una advertencia de cierre del expediente. Ejemplo:

```json
{"expediente_id":42,"folder_id":7,"folder_status":"ABIERTO","expediente_status":"ABIERTO","will_close_expediente":true}
```

`will_close_expediente` es verdadero solo cuando el expediente y la carpeta estan abiertos y no hay otra carpeta abierta del mismo expediente. Las carpetas ya cerradas no cuentan. Si la carpeta indicada ya esta cerrada, devuelve falso: repetir su cierre no cierra el expediente, incluso si fue reabierto sin reabrir sus carpetas.

La consulta valida la pertenencia de la carpeta y calcula todo en una sola sentencia SQL, sin modificar estados, fechas ni historial. IDs invalidos devuelven 400; expediente inexistente o carpeta inexistente/ajena, 404. No incluye contenido ni JSON internos de auditoria. Devuelve `Cache-Control: no-store`. El resultado refleja el momento de la consulta; el PATCH de cierre vuelve a evaluar el estado dentro de su transaccion si hubo cambios concurrentes.
