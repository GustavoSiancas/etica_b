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

La carpeta principal creada automaticamente comienza con `ABIERTO` y `DIRECCION DE ETICA`. El expediente no admite `status` ni `location` en su creacion.

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

`GET /expedientes?limit=20&offset=0` devuelve `{data,total,limit,offset}` ordenado por creación descendente e ID. `limit` admite 1–100 y `offset` desde cero. El estado y la ubicacion se consultan en las carpetas del expediente.

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

El expediente ya no contiene `status` ni `location`. La migracion `008_folder_state_location.up.sql` copia ambos valores historicos a la carpeta llamada `EXPEDIENTE PRINCIPAL` y retira las columnas del expediente. Las demas carpetas existentes comienzan en `ABIERTO` y `DIRECCION DE ETICA`. Los nuevos expedientes crean su carpeta principal con esos mismos valores por defecto.

- `GET /folder-statuses`: `ABIERTO`, `CERRADO`.
- `GET /folder-locations`: `DIRECCION DE ETICA`, `CONSEJO DE ETICA`, `TRIBUNAL DE HONOR`.
- `PATCH /expedientes/{expedienteID}/folders/{folderID}/status`: `{"status":"CERRADO"}`; enviar `ABIERTO` para reabrir.
- `PATCH /expedientes/{expedienteID}/folders/{folderID}/location`: `{"location":"CONSEJO DE ETICA"}`.

Ambos PATCH devuelven 200 con la carpeta actualizada, 400 para valores o IDs invalidos y 404 para expediente inexistente o carpeta ajena/inexistente. Cambiar ubicacion conserva el estado y esta permitido aunque la carpeta este cerrada. Las anteriores rutas de estado y ubicacion del expediente fueron retiradas.

Una carpeta cerrada bloquea con 409 cambios de sus secciones y vinculos. Las otras carpetas siguen siendo independientes. Los documentos compartidos se protegen contra edicion, eliminacion, firma y confirmacion de carga si cualquiera de sus carpetas esta cerrada. Las consultas permanecen disponibles. Cerrar una carpeta no bloquea los datos generales del expediente, sus participantes ni la creacion de carpetas adicionales.

Los filtros `status` y `location` de `GET /expedientes` y de las listas por participante buscan coincidencias en una misma carpeta; devuelven cada expediente una sola vez. `from` y `to` siguen filtrando la fecha de apertura del expediente.

## Actualizar descripcion

`PATCH /expedientes/{id}/description`

```json
{"description":"Nueva descripcion del expediente"}
```

Devuelve 200 con el expediente actualizado y `updated_at` en UTC. El campo es obligatorio, debe ser texto y se recortan los espacios externos. Enviar `""` borra la descripcion; omitirlo, enviar `null`, un ID invalido o campos desconocidos devuelve 400. Devuelve 404 si el expediente no existe. El estado de las carpetas no bloquea esta operacion.
