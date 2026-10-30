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

El expediente tiene estado `ABIERTO` o `CERRADO` y ubicación `DIRECCION DE ETICA`, `CONSEJO DE ETICA` o `TRIBUNAL DE HONOR`, restringidos mediante CHECK en SQL. Los valores iniciales son `ABIERTO` y `DIRECCION DE ETICA`. Los servicios de la aplicación bloquean cambios de información cuando está cerrado, incluidos documentos compartidos y cargas; se permite reabrir mediante el endpoint de estado.

`expediente_section_parent_id = NULL` identifica una sección raíz. Cada sección puede tener varias hijas, sin límite fijo de profundidad. Una FK compuesta garantiza que el padre pertenezca al mismo expediente. Se impiden padres propios y ciclos indirectos mediante una restricción y un trigger. Las modificaciones se serializan por expediente; si el nivel de aislamiento produce errores de serialización, el cliente debe reintentar la transacción.

`sort_order` ordena entre secciones con el mismo padre, incluidas las raíces. Admite enteros desde cero y empates, resueltos por ID.

Ejemplo: expediente → Denuncia → Documentación presentada → Anexos → Fotografías. Cualquiera de esos niveles puede contener documentos.

Para borrar una sección con hijas primero deben moverse o eliminarse sus hijas. Borrar una sección elimina sus vínculos y conserva los documentos. Un documento puede vincularse a varias secciones, incluso de distintos expedientes.

El esquema no crea escritos, sus relaciones, etapas ni subetapas. La reversión `001_initial.down.sql` elimina todas las tablas y sus datos.

Las entidades, endpoints, limpieza y datos de prueba usan este esquema. Consultar el contrato en [secciones y documentos](documents.md).
