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
| PATCH | `/documents/{id}` | Cambia nombre o enlace (200) |
| DELETE | `/documents/{id}` | Elimina documento sin vínculos (204) |
| GET | `/expedientes/{id}/documents` | Documentos únicos de todas las secciones del expediente (200) |

```json
{"name":"Denuncia firmada.pdf","file_url":"https://example.com/denuncia.pdf"}
```

`name` es obligatorio, no vacío y admite hasta 255 caracteres. `file_url` es una URL HTTPS opcional. La respuesta incluye ID, nombre, enlace y auditoría. PATCH con `file_url: null` limpia el enlace; campos omitidos conservan su valor. Cambiar un documento compartido afecta a todas las secciones donde se consulta. DELETE devuelve 409 si el documento sigue vinculado a alguna sección. Eliminar el registro no elimina archivos de R2.

La lista global devuelve `{data,total,limit,offset}`, ordenada por creación descendente e ID. `limit` acepta de 1 a 100 y `offset` desde cero. La lista del expediente devuelve un array ordenado por creación ascendente e ID, sin duplicar documentos vinculados a varias secciones.

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

Se retiraron `/escritos` y todas sus rutas, `/document-stages` y `POST /expedientes/{id}/documents`. Ya no se reciben `title`, `stage`, `type`, `status` ni `document_date` en el documento. Los campos desconocidos se rechazan con 400. Los participantes se siguen consultando desde el expediente.

Los IDs inválidos devuelven 400; expedientes, secciones, documentos o vínculos inexistentes devuelven 404. Las rutas de sección verifican siempre que la sección pertenezca al expediente indicado.

Si el expediente está `CERRADO`, las mutaciones de secciones y vínculos devuelven 409. Los documentos y sus cargas también devuelven 409 si cualquiera de sus expedientes está cerrado. Las consultas siguen disponibles; reabrir mediante `PATCH /expedientes/{id}/status` permite volver a ingresar información. Ver [estado y ubicación](expedientes.md).
