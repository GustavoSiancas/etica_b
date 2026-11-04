# Secciones y subsecciones del expediente

El esquema está consolidado en `migrations/001_initial.up.sql`. Se instala una sola vez sobre una base nueva; para bases existentes, `002_expediente_state_location.up.sql` a?ade ubicaci?n y normaliza estados al arrancar. No se ha aplicado a la base configurada.

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_initial.up.sql
```

- `documents`: ID, nombre, enlace opcional y auditoría.
- `expediente_section`: expediente, padre opcional, nombre, tipo libre, fecha opcional, orden, color hexadecimal y auditoría.
- `expediente_section_documents`: relación entre documentos y secciones, sin vínculos duplicados.
- `expediente_year_counters`: conserva la numeración anual automática.
- `document_types`: conserva sugerencias de tipo, sin FK desde las secciones.

Cada carpeta tiene estado `ABIERTO` o `CERRADO` y ubicacion `DIRECCION DE ETICA`, `CONSEJO DE ETICA` o `TRIBUNAL DE HONOR`. La migracion 008 hereda los valores historicos del expediente en su carpeta `EXPEDIENTE PRINCIPAL` y retira las columnas del expediente. El cierre protege solo el contenido de esa carpeta; un documento compartido se protege si alguna carpeta propietaria esta cerrada.

`expediente_section_parent_id = NULL` identifica una sección raíz. Cada sección puede tener varias hijas, sin límite fijo de profundidad. Una FK compuesta garantiza que el padre pertenezca al mismo expediente. Se impiden padres propios y ciclos indirectos mediante una restricción y un trigger. Las modificaciones se serializan por expediente; si el nivel de aislamiento produce errores de serialización, el cliente debe reintentar la transacción.

`sort_order` ordena entre secciones con el mismo padre, incluidas las raíces. Admite enteros desde cero y empates, resueltos por ID.

Ejemplo: expediente → Denuncia → Documentación presentada → Anexos → Fotografías. Cualquiera de esos niveles puede contener documentos.

Para borrar una sección con hijas primero deben moverse o eliminarse sus hijas. Borrar una sección elimina sus vínculos y conserva los documentos. Un documento puede vincularse a varias secciones, incluso de distintos expedientes.

El esquema no crea escritos, sus relaciones, etapas ni subetapas. La reversión `001_initial.down.sql` elimina todas las tablas y sus datos.

Las entidades, endpoints, limpieza y datos de prueba usan este esquema. Consultar el contrato en [secciones y documentos](documents.md).

## Carpetas (migraci?n 004)

La jerarqu?a es expediente ? carpeta ? secci?n ? subsecciones ? documentos.

- `folder_types`: cat?logo de nombres; valores iniciales `EXPEDIENTE PRINCIPAL` y `EXPEDIENTE CAUTELAR`.
- `expediente_folders`: `id`, `expediente_id`, `folder_type_id` y auditor?a. Cada expediente admite una carpeta por tipo.
- `expediente_section.expediente_folder_id`: obligatorio. Se conserva `expediente_id` para compatibilidad y se verifica con una FK compuesta que la carpeta pertenezca al expediente. Otra FK obliga a que padre e hija est?n en la misma carpeta.

La migraci?n 004 se aplica autom?ticamente al arrancar, en una transacci?n y una sola vez. Crea una carpeta principal para cada expediente existente y asigna todas sus secciones y subsecciones a ella, sin cambiar IDs, padres ni v?nculos con documentos. Los documentos compartidos conservan todos sus v?nculos. Los documentos independientes no tienen expediente y permanecen independientes. Los expedientes nuevos reciben autom?ticamente su carpeta principal.

### API

- `GET /folder-types`: cat?logo de nombres e IDs.
- `GET /expedientes/{id}/folders`: carpetas del expediente, incluyendo el nombre del cat?logo.
- `POST /expedientes/{id}/folders` con `{"folder_type_id":2,"opening_date":"2026-11-04"}`: crea una carpeta independiente; admite `status` y `location` opcionales. Si ya existe devuelve la misma sin alterar sus valores.
- `GET /expedientes/{id}/folders/{folderID}/sections`: secciones y subsecciones de esa carpeta.
- `POST /expedientes/{id}/sections`: admite `expediente_folder_id`. Si se omite, hereda la carpeta del padre o utiliza la principal para una ra?z. La respuesta de secci?n incluye `expediente_folder_id`.

Las rutas anteriores siguen disponibles. Cambiar el padre solo est? permitido dentro de la misma carpeta; no se implementa traslado de ?rboles entre carpetas.
