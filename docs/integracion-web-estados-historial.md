# Integración web: estados, cierres e historial

Resumen de los cambios del backend para integrar en el frontend. Todas las rutas son relativas a la URL base actual de la API. Los ejemplos usan IDs ilustrativos.

## 1. Estados y ubicación

El expediente tiene su propio `status`. Cada carpeta tiene su propio `status` y `location`.

| Campo | Valores |
| --- | --- |
| `status` | `ABIERTO`, `CERRADO` |
| `location` de carpeta | `DIRECCION DE ETICA`, `CONSEJO DE ETICA`, `TRIBUNAL DE HONOR` |

La ubicación es independiente del estado. No hay ubicación ni endpoint de ubicación en el expediente.

Catálogos existentes:

- `GET /folder-statuses`: arreglo de objetos `{value,label}`.
- `GET /folder-locations`: arreglo de objetos `{value,label}`.

## 2. Fechas y respuestas

Las fechas de apertura existentes se conservan al cerrar o reabrir. El backend asigna las fechas de cierre; no enviarlas desde la web.

| Recurso | `opening_date` y `closing_date` |
| --- | --- |
| Expediente | Fecha calendario `YYYY-MM-DD`; cierre calculado según `America/Lima` |
| Carpeta | Timestamp ISO 8601, devuelto en UTC |
| Entrada del historial | Timestamp ISO 8601 en UTC; fechas calendario del expediente representadas como medianoche de Perú |

Una fecha ausente se devuelve como `null`. Para timestamps, mostrar la hora de Perú en la interfaz. Para fechas calendario, evitar convertir `YYYY-MM-DD` como si fuera un instante UTC: podría mostrarse el día anterior.

Ejemplo de expediente devuelto por los PATCH y en los listados:

```json
{
  "id": 42,
  "number": "1-2026",
  "description": "Caso de ejemplo",
  "init_type": "DENUNCIA_DE_PARTE",
  "status": "ABIERTO",
  "opening_date": "2026-11-01",
  "closing_date": null,
  "created_at": "2026-11-01T15:00:00Z",
  "updated_at": "2026-11-09T15:00:00Z"
}
```

Ejemplo de carpeta:

```json
{
  "id": 7,
  "expediente_id": 42,
  "folder_type_id": 1,
  "name": "EXPEDIENTE PRINCIPAL",
  "abbreviation": "EP",
  "status": "ABIERTO",
  "location": "DIRECCION DE ETICA",
  "opening_date": "2026-11-01T05:00:00Z",
  "closing_date": null,
  "created_at": "2026-11-01T15:00:00Z",
  "updated_at": "2026-11-09T15:00:00Z"
}
```

`GET /expedientes/{expedienteID}/folders` devuelve un arreglo de carpetas. Los listados paginados `GET /expedientes` y `GET /folders` devuelven `{data,total,limit,offset}`. El listado global de carpetas también incluye los campos existentes `expediente_number` y `denomination`.

## 3. Abrir y cerrar

| Acción | Método y ruta | Body | Respuesta 200 |
| --- | --- | --- | --- |
| Cerrar expediente | `PATCH /expedientes/{id}/status` | `{"status":"CERRADO"}` | Expediente actualizado |
| Reabrir expediente | `PATCH /expedientes/{id}/status` | `{"status":"ABIERTO"}` | Expediente actualizado |
| Cerrar carpeta | `PATCH /expedientes/{expedienteID}/folders/{folderID}/status` | `{"status":"CERRADO"}` | Carpeta actualizada |
| Reabrir carpeta | Misma ruta de estado de carpeta | `{"status":"ABIERTO"}` | Carpeta actualizada |
| Cambiar ubicación | `PATCH /expedientes/{expedienteID}/folders/{folderID}/location` | `{"location":"CONSEJO DE ETICA"}` | Carpeta actualizada |

Enviar `Content-Type: application/json`. Los bodies de estado y ubicación no aceptan campos adicionales.

### Reglas que debe reflejar la web

1. **Cerrar expediente:** cierra todas sus carpetas abiertas y asigna la fecha actual. Las carpetas ya cerradas conservan su fecha de cierre y `updated_at`.
2. **Cerrar carpeta:** asigna su propia fecha de cierre. Si era la última abierta, también cierra el expediente y asigna la fecha de cierre de este.
3. **Reabrir expediente:** elimina únicamente su fecha de cierre y lo pone en `ABIERTO`. No reabre carpetas ni cambia las fechas de ellas. Por tanto, puede existir un expediente abierto con todas sus carpetas cerradas.
4. **Reabrir carpeta:** elimina su fecha de cierre. Si el expediente estaba cerrado, también lo reabre y elimina la fecha de cierre del expediente.
5. **Repetir el mismo estado:** conserva las fechas y no duplica eventos de apertura/cierre. Repetir el cierre de una carpeta ya cerrada no cierra un expediente reabierto por separado.
6. **Cambiar ubicación:** no cambia el estado ni la fecha de cierre. Se permite también en carpetas cerradas.
7. **Contenido de carpeta cerrada:** las operaciones de modificación del contenido pueden responder 409; reabrir la carpeta antes de editarlas.

Los cambios de estado y su historial se guardan en la misma transacción. El cierre definitivo lo decide el backend.

### Creación

- El expediente nuevo comienza en `ABIERTO` y crea automáticamente su carpeta principal abierta en `DIRECCION DE ETICA`. La creación del expediente no acepta `status` ni `location`.
- Al crear una carpeta, `status` y `location` son opcionales y sus valores predeterminados son `ABIERTO` y `DIRECCION DE ETICA`.
- Crear una carpeta abierta reabre el expediente si estaba cerrado. Crear una carpeta cerrada asigna su fecha de cierre actual.
- Crear de nuevo un tipo de carpeta que ya existe en el expediente devuelve la carpeta existente sin cambiar su estado ni fechas.

## 4. Consulta antes de cerrar una carpeta

```http
GET /expedientes/42/folders/7/status/closed
```

Respuesta 200:

```json
{
  "expediente_id": 42,
  "folder_id": 7,
  "folder_status": "ABIERTO",
  "expediente_status": "ABIERTO",
  "will_close_expediente": true
}
```

`will_close_expediente` es `true` cuando:

- El expediente está abierto.
- La carpeta indicada está abierta.
- No existe otra carpeta abierta en ese expediente.

Las carpetas cerradas no cuentan. Si la carpeta indicada ya está cerrada, devuelve `false`.

La web no necesita descargar todas las carpetas ni calcular cuál es la última: este GET resuelve la consulta con una sola sentencia SQL. No modifica estados, fechas ni historial. Devuelve `Cache-Control: no-store`.

### Flujo sugerido para el botón «Cerrar carpeta»

1. Consultar este GET cuando el usuario pulse el botón, sin sondeo periódico.
2. Si devuelve `true`, advertir: «Esta es la última carpeta abierta. Al cerrarla también se cerrará el expediente».
3. Si confirma, enviar el PATCH de estado con `CERRADO`.
4. Aplicar la carpeta devuelta y refrescar o invalidar los datos del expediente y los historiales visibles.

El GET es una previsión del estado actual. Otro usuario podría cambiar las carpetas antes del PATCH; el backend vuelve a evaluar las condiciones dentro de la transacción. No tratar la previsión como una garantía ni enviar el booleano al PATCH.

Después de cerrar un expediente, refrescar también sus carpetas, porque la respuesta del PATCH contiene solo el expediente. Después de reabrirlo, conservar los estados de las carpetas.

## 5. Historial de expediente y carpetas

| Consulta | GET |
| --- | --- |
| Todos los eventos de un expediente y sus carpetas | `/expedientes/{id}/history?limit=20&offset=0` |
| Solo los eventos de una carpeta | `/expedientes/{id}/folders/{folderID}/history?limit=20&offset=0` |

Paginación: `limit` entre 1 y 100, predeterminado 20; `offset` desde 0, predeterminado 0. Orden: `changed_at DESC, id DESC`. Un historial sin eventos devuelve `data: []` y `total: 0`.

Ejemplo de respuesta de historial de carpeta:

```json
{
  "data": [
    {
      "id": 125,
      "expediente_id": 42,
      "folder_id": 7,
      "folder_name": "EXPEDIENTE PRINCIPAL",
      "event": "CARPETA_CERRADA",
      "status": "CERRADO",
      "location": "CONSEJO DE ETICA",
      "opening_date": "2026-11-01T05:00:00Z",
      "closing_date": "2026-11-09T16:30:00Z",
      "changed_at": "2026-11-09T16:30:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

En eventos del expediente, `folder_id`, `folder_name` y `location` son `null`. La ubicación también puede ser `null` en eventos antiguos de carpeta, anteriores a su registro histórico; mostrar «Sin información histórica», sin reemplazarla con la ubicación actual.

| `event` | Etiqueta sugerida |
| --- | --- |
| `EXPEDIENTE_ABIERTO` | Expediente abierto/reabierto |
| `EXPEDIENTE_CERRADO` | Expediente cerrado |
| `EXPEDIENTE_DESCRIPCION_ACTUALIZADA` | Descripción actualizada |
| `CARPETA_ABIERTA` | Carpeta abierta/reabierta |
| `CARPETA_CERRADA` | Carpeta cerrada |
| `CARPETA_UBICACION_ACTUALIZADA` | Ubicación actualizada |

Los eventos de apertura cubren tanto la creación como la reapertura. Usar `changed_at` para mostrar cuándo ocurrió el evento: `opening_date` conserva la fecha de apertura original y no se reemplaza al reabrir.

### Información interna que no se entrega a la web

El backend conserva dos JSON para auditoría:

- `active_folders`: IDs y nombres de las carpetas abiertas al momento del evento.
- `content_snapshot`: copia de secciones, subsecciones y documentos al momento del evento de carpeta. Incluye IDs, nombres, relaciones padre/hijo y otros metadatos.

**Ninguno de esos JSON se expone en los GET.** La web no debe enviar, solicitar ni construir esos campos. No hay endpoints públicos para crear, editar o eliminar registros del historial.

El historial registra eventos desde la instalación de esta versión. No reconstruye eventos pasados. Las modificaciones del contenido no generan por sí solas un evento: el JSON guarda una copia cuando se registra el evento de estado o ubicación. Los cambios posteriores del contenido no alteran las copias anteriores.

## 6. Filtros existentes: detalle importante

En `GET /expedientes` y los listados por participante, los filtros `status` y `location` siguen buscando coincidencias en una misma **carpeta**, aunque el expediente ahora también devuelve su propio `status`. No interpretar `GET /expedientes?status=CERRADO` como un filtro exclusivo del estado del expediente.

Los filtros `from` y `to` del listado de expedientes siguen usando su fecha de apertura.

## 7. Errores

Formato de error:

```json
{"error":"folder not found in expediente"}
```

| HTTP | Significado para esta integración |
| --- | --- |
| 400 | ID, estado, ubicación, body o paginación inválidos |
| 404 | Expediente inexistente o carpeta inexistente/ajena al expediente |
| 409 | Conflicto; por ejemplo, intento de modificar contenido de carpeta cerrada |
| 500 | Error interno; no asumir que la operación se completó |

No actualizar la interfaz como si la operación hubiera sido exitosa cuando falle el PATCH. Los GET de historial y previsión son de solo lectura.

## 8. Disponibilidad y referencias

El arranque del backend registra las migraciones en orden:

- `009_expediente_status_closing_dates.up.sql`: estado del expediente y fechas de cierre.
- `010_expediente_history.up.sql`: historial y JSON de carpetas activas.
- `011_folder_history_details.up.sql`: ubicación histórica y JSON del contenido de carpeta.

La previsión de cierre no requiere una migración adicional. Estas funciones se verificaron con pruebas de PostgreSQL, incluidos cierres simultáneos, conservación de fechas, privacidad de los JSON y reversión de cambios cuando falla el historial.

Contrato completo: [OpenAPI](../internal/httpapi/openapi.json). Documentación del recurso: [Expedientes](expedientes.md).
