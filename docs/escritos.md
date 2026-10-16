# Escritos

Un escrito es un registro informativo independiente. `expediente_number` es solo texto para indicar a que expediente apunta; no es una FK y el servicio no consulta expedientes ni participantes. `type` es texto libre enviado por el frontend. Los remitentes se asocian por IDs de denunciantes y denunciados existentes, mediante las tablas intermedias `escrito_denunciantes` y `escrito_denunciados`. Se permite uno o varios; cada escrito debe tener al menos un remitente.

Cada escrito tiene titulo, tipo, numero informativo del expediente, estado, remitentes y fechas de auditoria. La fecha del documento y el archivo son opcionales. Al crear comienza con estado `NUEVA`.

## Crear escrito

`POST /escritos`

```json
{
  "title": "Solicitud de atencion",
  "type": "SOLICITUD",
  "expediente_number": "1-2026",
  "document_date": "2026-10-16",
  "denunciante_ids": [4],
  "denunciado_ids": [12]
}
```

`title`, `type`, `expediente_number` y al menos un remitente entre `denunciante_ids` y `denunciado_ids` son obligatorios. `file_url` tambien se puede enviar como URL HTTPS, o se puede adjuntar despues con el flujo de subida.

## Listar escritos

`GET /escritos?status=NUEVA&limit=20&offset=0`

Devuelve una pagina con `data`, `total`, `limit` y `offset`. El filtro opcional `status` acepta `NUEVA` o `ATENDIDA`; si se omite, lista ambos estados. Ordena por llegada: `created_at ASC` y, para desempatar, `id ASC`. `limit` por defecto es 20, acepta 1-100; `offset` por defecto es 0 y no puede ser negativo. Un estado desconocido devuelve `400`.

## Cambiar estado

`PATCH /escritos/{id}/status`

```json
{"status":"ATENDIDA"}
```

El PATCH solo acepta `ATENDIDA`: el unico cambio permitido es `NUEVA` ? `ATENDIDA`. No se puede volver a `NUEVA`; si ya esta `ATENDIDA`, responde `409 Conflict`. ID inexistente responde `404`. La respuesta incluye los remitentes asociados.

## Adjuntar archivo opcional con R2

El escrito debe existir primero. El frontend pide una firma al backend, sube el archivo directamente a R2 y confirma la URL con el backend:

1. `POST /escritos/{id}/upload-url`, cuerpo `{"content_type":"application/pdf"}`. Tipos admitidos: PDF, DOC, DOCX, PNG, JPEG, WebP y GIF. Limite 10 MiB; firma valida cinco minutos.
2. Subir el binario con `PUT` a `upload_url`, usando exactamente los `headers` de la respuesta.
3. `PATCH /escritos/{id}/file-url`, cuerpo `{"file_url":"<file_url recibido>"}`. El backend verifica que el archivo exista, sea de un tipo admitido, no exceda el limite y pertenezca a la ruta del escrito; luego guarda la URL.

El archivo es opcional y se puede adjuntar despues de crear el escrito. El backend no recibe el binario; el navegador lo envia directamente a R2.
