# Persistencia del módulo de Ética CAL

Backend Go/GORM y PostgreSQL 12 o superior con expedientes, participantes, secciones jerárquicas, documentos compartidos y carga de archivos a R2.

## Configuración y arranque

Completar `.env` según `.env.example` con PostgreSQL y R2. Las variables del sistema prevalecen sobre `.env`. No publicar credenciales. Los modelos están en `internal/persistence/models` y la migración SQL es la fuente de verdad; no usar AutoMigrate.

```powershell
go mod download
go run ./cmd/app
```

El arranque verifica PostgreSQL y R2 y escucha por defecto en `:8080`. Instala `migrations/001_initial.up.sql` únicamente si no existe `public.expedientes`. La migración consolidada es para bases nuevas; una base con el esquema anterior requiere actualización antes de utilizar esta versión. No se han modificado bases existentes.

También se puede instalar manualmente en una base nueva:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_initial.up.sql
```

Swagger: `http://localhost:8080/swagger/`. OpenAPI: `http://localhost:8080/openapi.json`. Swagger UI carga sus recursos estáticos desde un CDN.

## Flujo actual

1. Consultar denunciantes y denunciados existentes. Sus POST solo se habilitan con `ENABLE_INITIAL_PARTICIPANT_LOAD=true` para la carga inicial.
2. Consultar `GET /tipos-inicio` y crear el expediente con descripción, fecha de apertura, tipo de inicio y listas de IDs de participantes. Las asociaciones se guardan en la misma transacción y son permanentes.
3. Crear secciones con `POST /expedientes/{id}/sections`. Para crear subsecciones, enviar `expediente_section_parent_id`. No hay límite fijo de profundidad; se impiden ciclos y padres de otro expediente.
4. Crear documentos dentro de una sección o crearlos independientemente y vincularlos. Cada documento guarda nombre y enlace; la sección guarda nombre, tipo, fecha, orden y color.
5. Para adjuntos de R2, solicitar URL de carga, subir el archivo y confirmar el enlace. Se admiten imágenes, PDF, DOC y DOCX de hasta 10 MiB.

El contador `expediente_year_counters` genera automáticamente `number`, por ejemplo `1-2026`, según el año de la fecha de apertura (`opening_date`). Cada año tiene su propio consecutivo. No existe el campo `code`. `GET /document-types` se conserva como catálogo de sugerencias para el tipo libre de sección. Escritos, etapas y subetapas ya no tienen entidades ni endpoints.

La migración `003_expediente_opening_year.up.sql` se aplica al arrancar después de la 002. Cambia el año usado por el trigger, conserva los números existentes y sincroniza los contadores con los números ya asignados para evitar duplicados. No renumera expedientes históricos. Las nuevas inserciones requieren fecha de apertura.

El estado y la ubicacion pertenecen a cada carpeta. La migracion `008_folder_state_location.up.sql` copia los valores anteriores a `EXPEDIENTE PRINCIPAL` y elimina esos campos del expediente. Las demas carpetas comienzan en `ABIERTO` y `DIRECCION DE ETICA`. La migracion `009_expediente_status_closing_dates.up.sql` devuelve al expediente su propio estado y agrega fechas de cierre a expedientes y carpetas. `PATCH /expedientes/{id}/status` cierra las carpetas abiertas conservando las fechas de las ya cerradas; reabrir el expediente solo limpia su propia fecha de cierre. Cerrar la ultima carpeta abierta tambien cierra el expediente. Los catalogos son `GET /folder-statuses` y `GET /folder-locations`; se actualizan mediante `PATCH /expedientes/{expedienteID}/folders/{folderID}/status` o `/location`.

Cerrar una carpeta conserva las consultas y permite cambiar su ubicacion, pero bloquea con 409 sus secciones, vinculos y documentos, incluidas firmas y confirmaciones de carga. Las otras carpetas siguen siendo independientes. Un documento compartido queda protegido si cualquiera de sus carpetas esta cerrada. Los datos generales del expediente permanecen editables.

## Contratos

- [Expedientes](docs/expedientes.md): crear, listar, tipos de inicio y estado.
- [Secciones y documentos](docs/documents.md): CRUD, jerarquía y vínculos.
- [Participantes](docs/participants.md): consulta, filtros y carga inicial.
- [Expedientes por participante](docs/expedientes-por-participante.md): ficha de denunciante o denunciado con expedientes paginados y filtros de estado y ubicación.
- [Carga de archivos](docs/uploads.md): firma temporal y confirmación de R2.
- [ERD](docs/erd.md) y [reglas de esquema](docs/expediente-sections-schema.md).

## Carga inicial de colegiados

Al arrancar, despues de comprobar la migracion y antes de iniciar HTTP, la aplicacion ejecuta `seeds/demo_data.sql`. Ese archivo solo inserta denunciados desde el CSV de colegiados; no genera expedientes, secciones ni documentos. Esta carga de arranque siempre se ejecuta y no depende de ENABLE_DEMO_RESET.

El servicio Python `scripts/import_participants.py` genera el SQL sin conectarse a PostgreSQL. Solo requiere Python 3.10 o superior, sin paquetes adicionales:

```powershell
python scripts/import_participants.py 'C:\ruta\results.csv' --skip-invalid
```

Columnas: `numero_de_cal`, `nombre_completo`, `tipo_documento` (DNI/CE) y `numero_documento`. Se detecta el separador CSV y se usa UTF-8 por defecto; otras codificaciones pueden indicarse con `--encoding`. Los documentos se conservan como texto, incluidos ceros iniciales y longitudes originales. No se completan digitos inventados.

El SQL generado se carga en cada arranque; se omiten colegiaturas ya existentes sin modificar sus datos. Una tabla temporal y un bloqueo transaccional evitan duplicados entre arranques concurrentes. Distintas colegiaturas con el mismo documento se conservan como en la fuente. Los importados no se marcan como is_demo.

El arranque también ejecuta `seeds/demo_data_denuncianes.sql`: contiene 50 denunciantes de muestra (25 empresas reales y 25 figuras públicas históricas), con documentos ficticios: 25 RUC para empresas, 20 DNI y cinco CE para personas. No los vincula a expedientes y los marca con `is_demo=true`. Completa el documento de registros demo anteriores si todav?a est? vac?o; conserva documentos ya registrados y datos no demo. Omite nombres ya existentes, ignorando mayúsculas y espacios exteriores, para no duplicarlos al reiniciar. La recarga de desarrollo ejecuta ambos archivos y devuelve los totales de denunciados y denunciantes.

Sin `--skip-invalid`, cualquier fila invalida detiene la generacion y conserva el SQL anterior. El reporte predeterminado es `.cache/import_rejected.csv`; `--report` permite otra ruta. El CSV proporcionado contiene 90 138 filas: se generaron 90 131 registros validos y se apartaron siete filas para revision. El reporte contiene datos personales y queda dentro de `.cache`, excluido de Git.

Tambien puede ejecutarse el SQL manualmente:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f seeds/demo_data.sql
```

`GET /dev/demo-data/reset`, con `ENABLE_DEMO_RESET=true`, ahora solo vuelve a ejecutar la misma insercion; conserva su ruta por compatibilidad y no borra datos. Su respuesta `denunciados` indica el total de registros despues de la carga. `DELETE /dev/database/clear`, con `ENABLE_DATABASE_CLEAR=true`, sigue eliminando todos los registros y reiniciando identidades. En el siguiente arranque se cargan nuevamente los colegiados. Estas rutas se mantienen solo para desarrollo/pruebas.

`001_initial.down.sql` elimina todo el esquema y sus datos; usar solo en bases desechables.

## Verificación

```powershell
go test ./...
go vet ./...
```

Las pruebas de servicios contra PostgreSQL se habilitan con `ETICA_INTEGRATION=1` y las variables `PG_*` de una base de prueba. Las pruebas HTTP completas requieren `ETICA_TEST_DATABASE_URL` con una URL PostgreSQL de prueba. Crean y eliminan esquemas aislados; no usan tablas de la aplicación.

## Logs

Cada petición genera un log JSON con `request_id`, método, ruta, estado, duración y bytes. No se registran cuerpos, cabeceras ni consultas. Los detalles internos de errores no se envían al cliente. Las respuestas incluyen `X-Request-ID` y CORS permite todos los orígenes.

## Actualizar estado y ubicacion en una base existente

El error SQLSTATE 42703 al actualizar la ubicacion indica que falta la columna location. Reiniciar `go run ./cmd/app` aplica automaticamente la migracion 002 antes de cargar participantes y abrir HTTP. Tambien puede ejecutarse manualmente:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/002_expediente_state_location.up.sql
```

Conserva expedientes, documentos, participantes y numeracion. OPEN e IN_PROGRESS se convierten en ABIERTO; CLOSED se convierte en CERRADO. Las ubicaciones ausentes se inicializan como DIRECCION DE ETICA y las existentes se conservan. Si hay estados personalizados desconocidos o ubicaciones fuera de las opciones permitidas, la transaccion se revierte para permitir corregirlos sin descartar informacion. La aplicacion requiere permisos para modificar esas tablas. No hay reversion automatica de 002 porque la conversion agrupa OPEN e IN_PROGRESS y no puede reconstruir el estado original.


La migracion `010_expediente_history.up.sql` agrega el historial interno de expedientes y carpetas. `GET /expedientes/{id}/history?limit=20&offset=0` permite consultar eventos y fechas; el JSONB de carpetas activas se conserva exclusivamente para auditoria y no se incluye en la respuesta. El historial comienza a registrar eventos con esta version. Ver `docs/expedientes.md`.

La migracion `011_folder_history_details.up.sql` agrega la ubicacion historica y una copia JSON interna de secciones, subsecciones y documentos por evento de carpeta. `GET /expedientes/{id}/folders/{folderID}/history` permite consultar estados, ubicaciones y fechas, sin exponer los JSON de auditoria.
