# Persistencia del módulo de Ética CAL

Estructura mínima Go/GORM y PostgreSQL 12 o superior. Incluye endpoints de firma temporal de subida y confirmación de URL de imágenes. Los modelos están en `internal/persistence/models`. La migracion consolidada es la fuente de verdad: **no usar AutoMigrate**, pues no reproduce los triggers ni todas las restricciones.

## Configuración y arranque

Endpoints de expedientes: `GET /tipos-inicio` devuelve los valores enum disponibles; `POST /expedientes` crea enviando descripcion, tipo y fecha de apertura, y `GET /expedientes` lista. La fecha de apertura es independiente de `created_at`. El tipo vive directamente en `expedientes.init_type`, sin tabla catálogo. Ver [contrato y ejemplos PowerShell](docs/expedientes.md).

Los escritos se crean con `POST /escritos` indicando los remitentes por IDs de denunciantes o denunciados; se listan paginados en orden de llegada con `GET /escritos` y su estado se cambia con `PATCH /escritos/{id}/status`. El numero de expediente sigue siendo informativo, sin relacion FK. Ver [contrato y ejemplos](docs/escritos.md).

Los documentos se crean con `POST /expedientes/{id}/documents` y se listan ordenados por fecha con `GET /expedientes/{id}/documents`. Ver [contrato y ejemplos](docs/documents.md).

Denunciantes y denunciados se cargan inicialmente. Sus POST estan desactivados por defecto y solo se habilitan con ENABLE_INITIAL_PARTICIPANT_LOAD=true. Al crear el expediente se envian denunciante_ids y denunciado_ids; las asociaciones son permanentes. Ver [contrato](docs/participants.md).

Completar `.env` en la raíz con los datos PostgreSQL y R2; `.env.example` documenta las variables sin secretos. Las variables del sistema prevalecen sobre `.env`. `.gitignore` excluye `.env`, cachés y ejecutables. No se configura remoto ni se publica en GitHub.

En Cloudflare, crear un bucket R2 y generar credenciales de la API S3 con acceso de lectura/escritura a ese bucket. Copiar Account ID, Access Key ID, Secret Access Key y nombre del bucket a `.env`. No usar el token general de Cloudflare en lugar de las credenciales S3. El cliente usa región `auto` y endpoint `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`; `R2_ENDPOINT` permite especificar el endpoint de una jurisdicción especial. Referencia: [ejemplo oficial Go de Cloudflare](https://developers.cloudflare.com/r2/examples/aws/aws-sdk-go/).

Desde la raíz:

```powershell
go mod download
go run ./cmd/app
```

Swagger UI queda disponible en `http://localhost:8080/swagger/` y el documento OpenAPI en `http://localhost:8080/openapi.json`. La especificación se sirve localmente; Swagger UI carga sus archivos estáticos desde un CDN, así que el navegador requiere Internet para mostrar la interfaz.

El arranque verifica PostgreSQL y R2 y levanta HTTP en :8080. El esquema consolidado se instala una sola vez en una base nueva; el arranque no transforma esquemas anteriores. Completar también R2_PUBLIC_URL y FRONTEND_ORIGIN. Ver [flujo de imágenes, endpoints y ejemplo frontend](docs/uploads.md).

## Instalación

Con `psql` instalado y `DATABASE_URL` configurada, para una base nueva ejecutar el ?nico archivo consolidado:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f migrations/001_initial.up.sql
```

Para cargar el conjunto de datos ficticios (solo en pruebas), ejecutar `psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -f seeds/demo_data.sql`; crea tres expedientes, participantes, 45 documentos y seis escritos con enlaces PDF publicos. El archivo reemplaza ?nicamente filas `[DEMO]`; se puede volver a ejecutar sin duplicarlas y conserva los datos reales.

La ruta de desarrollo `GET /dev/demo-data/reset` vuelve a cargar el fixture y reemplaza ?nicamente filas `[DEMO]`. Est? deshabilitada por defecto; para activarla en un entorno local de pruebas, configurar `ENABLE_DEMO_RESET=true`. No habilitarla en producci?n.

El arranque de la aplicaci?n tambi?n aplica este esquema si no existe `public.expedientes`. La migraci?n incluye tablas, enum, triggers, funciones, ?ndices y el catalogo inicial de cinco etapas y quince subetapas, mas OTROS y el diccionario de tipos. Es transaccional y solo debe instalarse una vez.

`migrations/001_initial.down.sql` revierte el esquema **eliminando todos los datos**; ?sala ?nicamente en entornos desechables. No ejecutar en producci?n.

El esquema consolidado est? destinado a instalaciones nuevas. El arranque aplica `001_initial.up.sql` solo si la base a?n no tiene `public.expedientes`; una base ya existente no se modifica autom?ticamente.

## Relaciones e integridad

Cada documento pertenece a un expediente y guarda stage y type como texto libre. Las etapas, subetapas y tipos son diccionarios independientes para selectores. Las personas se pueden reutilizar entre expedientes y sus nombres no son únicos. Las tablas `expediente_denunciantes` y `expediente_denunciados` tienen ID propio y una restricción única por expediente/persona.

Los participantes pertenecen al expediente, con una unica relacion por expediente/persona. No hay tablas de participantes por documento, historial, estado activo/inactivo ni fecha de ingreso en las asociaciones. Los documentos no devuelven listas de participantes; consultar los endpoints de participantes del expediente.

## Verificacion

Ejecutar `go test ./...`. Las pruebas de PostgreSQL se habilitan con `ETICA_INTEGRATION=1` y usan esquemas aislados.

## Logs de HTTP

La consola del servicio registra cada peticion en JSON con `request_id`, metodo, path, ruta, status, duracion y bytes. Los errores 4xx usan WARN y los 5xx ERROR; los errores de servicio incluyen `error` para diagnostico interno. No se registran cuerpos, cabeceras ni parametros de consulta. Los detalles internos no se devuelven al cliente.

Cada respuesta incluye `X-Request-ID`, expuesto por CORS para que el frontend pueda mostrarlo o registrarlo al reportar un fallo. Buscar ese ID en los logs del despliegue permite correlacionar la respuesta con el backend. Si no existe un registro correspondiente, revisar tambien el proxy, el dominio y el despliegue que recibio la peticion.

Al arrancar se registra `demo_reset_enabled`. Si es `false`, `/dev/demo-data/reset` devuelve 404 y el log indica `demo data reset is disabled`. Habilitar `ENABLE_DEMO_RESET=true` solo en el servicio de desarrollo/pruebas y volver a desplegar o reiniciar para cargar la variable. Swagger documenta la ruta aunque este deshabilitada.

## Borrado completo para pruebas

`DELETE /dev/database/clear` elimina todos los registros de las tablas de la aplicacion, incluidos datos reales, contadores y catalogos de etapas/subetapas. Conserva las tablas, restricciones y triggers; reinicia las identidades y no inyecta datos. No elimina archivos almacenados en R2. Se habilita de forma independiente con `ENABLE_DATABASE_CLEAR=true` y esta deshabilitado por defecto (404). El borrado es transaccional; si falla, se revierte todo.

El servicio existente `GET /dev/demo-data/reset` sigue reemplazando solo filas `[DEMO]` y cargando el fixture con `ENABLE_DEMO_RESET=true`. El fixture repone automaticamente los catalogos faltantes antes de cargar documentos. Ambas rutas aparecen en Swagger y sus resultados se registran en consola. Estas rutas no tienen autenticacion: habilitarlas unicamente en entornos de desarrollo/pruebas.



El fixture incluye 100 denunciantes y 150 denunciados adicionales (104 y 154 en total). Los denunciantes incluyen personas y empresas ficticias sin CAL; los denunciados tienen numeros de colegiatura simulados en registration_number. Ningun participante muestra el prefijo [DEMO]. La columna interna `is_demo` identifica estas filas para el reset y sus conteos; la columna se define en el esquema consolidado. Los codigos no corresponden a colegiados verificados.

El reset crea un documento por subetapa activa y expediente (45 con el catalogo inicial de 5 etapas y 15 subetapas), ademas de seis escritos con remitentes. Hay 26 empresas entre los 104 denunciantes. Los adjuntos alternan tres PDF publicos de W3C y Adobe; son muestras externas sin relacion con los casos ficticios.

## Identificacion de participantes

Denunciantes: `name`, `document_type` (DNI/RUC/CE) y `document_number`. Denunciados: los mismos campos con DNI/CE y `registration_number` (colegiatura, entero positivo). Estos datos son obligatorios para nuevas altas. Los numeros de documento son texto numerico de hasta 20 caracteres para conservar ceros iniciales. Ver [contrato](docs/participants.md).

Todos estos campos estan incluidos en `migrations/001_initial.up.sql`.

## Descripcion y clasificacion de documentos

Los expedientes usan `description` (TEXT, opcional) en lugar de nombre/titulo. Si se omite se guarda como texto vacio. Los documentos conservan `title` y reciben `stage` y `type` como texto libre, incluyendo OTROS. `stage` puede contener una etapa general o una subetapa. Consultar `GET /document-stages` y `GET /document-types` para los selectores; no hay FK desde documentos a estos diccionarios.

El unico esquema de instalacion es `migrations/001_initial.up.sql`; `001_initial.down.sql` revierte esa misma instalacion. No quedan migraciones incrementales. Esta consolidacion no actualiza bases con un esquema anterior ni se ha aplicado a la base existente.

## Flujo de la aplicacion

1. Consultar los diccionarios con GET /document-stages y GET /document-types para los selectores.
2. Cargar inicialmente denunciantes/denunciados y luego desactivar ENABLE_INITIAL_PARTICIPANT_LOAD. Consultar sus GET para seleccionar participantes.
3. Crear el expediente con descripcion, fecha de apertura, tipo de inicio, denunciante_ids y denunciado_ids. El sistema genera su codigo y estado inicial y guarda todas las asociaciones en la misma transaccion; no hay altas posteriores por rutas separadas.
4. Crear documentos del expediente con titulo, stage y type como texto libre. No se copian participantes ni se exige una FK a los diccionarios.
5. Si corresponde, solicitar la URL de carga, subir el archivo y confirmar su URL con los endpoints existentes.

Los escritos conservan su flujo independiente: numero de expediente informativo, remitentes existentes y avance de NUEVA a ATENDIDA. Los endpoints de reset y limpieza siguen reservados a desarrollo mediante sus banderas de configuracion.
