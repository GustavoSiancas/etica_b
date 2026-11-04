# Documentos, secciones y subsecciones

Un expediente contiene secciones. Cada sección puede tener subsecciones sin límite fijo de profundidad y documentos asociados. Los documentos guardan `name` y `file_url`; tipo, fecha, orden y color se guardan en la sección.

## Secciones

| Método | Ruta | Resultado |
| --- | --- | --- |
| POST | `/expedientes/{id}/sections` | Crea raíz o subsección (201) |
| GET | `/expedientes/{id}/sections` | Lista todos los niveles (200) |
| GET | `/expedientes/{id}/sections/{section_id}` | Consulta una sección (200) |
| PATCH | `/expedientes/{id}/sections/{section_id}` | Edita, mueve u ordena (200) |
| DELETE | `/expedientes/{id}/sections/{section_id}` | Elimina una sección sin hijas (204) |

Crear raíz:

```json
{"name":"Denuncia","type":"OTROS","document_date":"2026-10-30","sort_order":0,"color_hex":"#2563EB"}
```

Crear subsección con el ID de su padre:

```json
{"name":"Anexos","type":"Documentación presentada","expediente_section_parent_id":1,"sort_order":0}
```

`name` y `type` son obligatorios y se recortan. `type` es texto libre, sin FK al catálogo. La fecha es opcional, con formato YYYY-MM-DD. El orden es entero desde cero y vale 0 por defecto. El color es `#RRGGBB`, se normaliza a mayúsculas y vale `#808080` por defecto. El padre omitido o NULL indica raíz. El padre debe existir en el mismo expediente; se rechazan padres propios y ciclos con 400.

La lista es un array plano con `id`, `expediente_id`, `expediente_section_parent_id`, los campos anteriores y auditoría. Incluye todos los niveles; el frontend construye el árbol mediante el ID del padre. Se ordena por padre (raíces primero), `sort_order` e ID. El orden se interpreta entre hermanas.

PATCH modifica solo los campos enviados. Ejemplo para mover a raíz y borrar fecha:

```json
{"expediente_section_parent_id":null,"document_date":null,"sort_order":0}
```

Un PATCH vacío devuelve 400. DELETE devuelve 409 si existen hijas: primero deben moverse o eliminarse. Eliminar una sección borra sus vínculos, conservando los documentos.

## Documentos independientes

| Método | Ruta | Resultado |
| --- | --- | --- |
| POST | `/documents` | Crea documento independiente (201) |
| GET | `/documents?limit=20&offset=0` | Lista paginada (200) |
| GET | `/documents/{id}` | Consulta documento (200) |
| PATCH | `/documents/{id}` | Cambia nombre, enlace o fecha (200) |
| DELETE | `/documents/{id}` | Elimina documento sin vínculos (204) |
| GET | `/expedientes/{id}/documents` | Documentos únicos de todas las secciones del expediente (200) |

```json
{"name":"Denuncia firmada.pdf","file_url":"https://example.com/denuncia.pdf"}
```

`name` es obligatorio, no vacío y admite hasta 255 caracteres. `file_url` es una URL HTTPS opcional. La respuesta incluye ID, nombre, enlace y auditoría. PATCH con `file_url: null` limpia el enlace; campos omitidos conservan su valor. Cambiar un documento compartido afecta a todas las secciones donde se consulta. DELETE devuelve 409 si el documento sigue vinculado a alguna sección. Eliminar el registro no elimina archivos de R2.

La lista global devuelve `{data,total,limit,offset}`, ordenada por creación descendente e ID. `limit` acepta de 1 a 100 y `offset` desde cero. La lista del expediente devuelve un array ordenado por creación ascendente e ID, sin duplicar documentos vinculados a varias secciones.

### Fechas de documentos y UTC

`document_date` es opcional en `POST /documents`, en la creacion dentro de una seccion y en `PATCH /documents/{id}`.

```json
{"name":"Documento.pdf","document_date":"2026-11-04"}
```

Una fecha `YYYY-MM-DD` representa las 00:00 de Peru (UTC-05:00): el ejemplo se guarda y devuelve como `2026-11-04T05:00:00Z`. Tambien admite RFC3339 con zona: `2026-11-04T14:30:00-05:00` se devuelve como `2026-11-04T19:30:00Z`. Las horas sin zona se rechazan con 400.

En PATCH, omitir `document_date` conserva su valor; `null` lo borra. Los documentos existentes mantienen `null`: no se infiere su fecha a partir de la creacion o de su seccion.

Los instantes usan `TIMESTAMPTZ` en PostgreSQL y UTC (`Z`) en las respuestas, incluidos `created_at` y `updated_at` de todos los recursos. `created_at` registra la creacion del registro; `document_date` es proporcionado por el cliente y no se asigna automaticamente al subir un archivo. Las fechas de calendario `opening_date` del expediente y `document_date` de la seccion conservan `YYYY-MM-DD`.

La migracion `005_document_date.up.sql` se aplica automaticamente al iniciar el backend.

## Documentos de una sección

Todas las rutas siguientes comienzan con `/expedientes/{id}/sections/{section_id}`:

| Método | Sufijo | Resultado |
| --- | --- | --- |
| POST | `/documents` | Crea y vincula documento en una transacción (201) |
| GET | `/documents` | Lista documentos directos de la sección (200) |
| POST | `/documents/{document_id}` | Vincula documento existente (204) |
| DELETE | `/documents/{document_id}` | Retira únicamente el vínculo (204) |

La creación usa el mismo cuerpo `{name,file_url}`. Vincular y desvincular no requieren cuerpo. Vincular es idempotente: repetir la petición no duplica el vínculo. Retirar un vínculo inexistente devuelve 404. Los documentos se pueden reutilizar entre secciones y expedientes. La lista directa no incluye documentos de subsecciones.

## Catálogo y carga

`GET /document-types` conserva sugerencias activas, ordenadas por `sort_order` e ID: Escrito de oficio, Resolución y OTROS. El texto seleccionado puede enviarse en `expediente_section.type`; se permiten valores propios.

Se mantienen `POST /documents/{id}/upload-url` y `PATCH /documents/{id}/file-url`. Admiten imágenes PNG/JPEG/WebP/GIF, PDF, DOC y DOCX, hasta 10 MiB. Ver [carga de archivos](uploads.md).

## Cambios de contrato

Se retiraron `/escritos` y todas sus rutas, `/document-stages` y `POST /expedientes/{id}/documents`. Ya no se reciben `title`, `stage`, `type`, `status` en el documento. Los campos desconocidos se rechazan con 400. Los participantes se siguen consultando desde el expediente.

Los IDs inválidos devuelven 400; expedientes, secciones, documentos o vínculos inexistentes devuelven 404. Las rutas de sección verifican siempre que la sección pertenezca al expediente indicado.

Si el expediente está `CERRADO`, las mutaciones de secciones y vínculos devuelven 409. Los documentos y sus cargas también devuelven 409 si cualquiera de sus expedientes está cerrado. Las consultas siguen disponibles; reabrir mediante `PATCH /expedientes/{id}/status` permite volver a ingresar información. Ver [estado y ubicación](expedientes.md).

## Fecha de apertura de carpetas

La migracion `006_folder_opening_date.up.sql` agrega `opening_date` (`TIMESTAMPTZ`) a las carpetas y se ejecuta al iniciar el backend.

- La carpeta `EXPEDIENTE PRINCIPAL` se crea automaticamente con la fecha de apertura del expediente, interpretada a las 00:00 de Peru. Si el expediente no tiene fecha, la carpeta conserva `null`.
- Las nuevas carpetas adicionales requieren su propia `opening_date`.
- Las carpetas adicionales existentes reciben su `created_at` como fecha inicial en la migracion.
- Crear un tipo de carpeta que ya existe devuelve la misma carpeta y conserva su fecha.

`POST /expedientes/{expedienteID}/folders`:

```json
{"folder_type_id":2,"opening_date":"2026-11-04"}
```

Tambien admite `"opening_date":"2026-11-04T14:30:00-05:00"`. La respuesta y `GET /expedientes/{expedienteID}/folders` incluyen `opening_date` en UTC: `2026-11-04T05:00:00Z` para el primer ejemplo. Las horas sin zona se rechazan con 400. La fecha de la carpeta principal se copia al crearla; posteriores cambios al expediente no modifican esa copia.

## Crear tipos de carpeta

`POST /folder-types` permite crear un tipo reutilizable en el catalogo. Por ahora es un endpoint abierto, sin autenticacion ni verificacion de administrador; la autorizacion se incorporara posteriormente.

```json
{"name":"Expediente especial"}
```

Respuesta 201:

```json
{"id":3,"name":"EXPEDIENTE ESPECIAL"}
```

`name` es obligatorio, de 1 a 255 caracteres; se recortan espacios externos y se normaliza a mayusculas. Nombres vacios o invalidos y campos desconocidos devuelven 400. Un nombre ya existente devuelve 409. `GET /folder-types` incluye los tipos creados.

Para crear una carpeta con ese tipo, enviar a `POST /expedientes/{expedienteID}/folders`:

```json
{"folder_type_id":3,"opening_date":"2026-11-04"}
```

Crear el tipo no crea carpetas automaticamente en los expedientes. No requiere una nueva migracion.
