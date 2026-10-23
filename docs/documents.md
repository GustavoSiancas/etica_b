# Documentos y diccionarios

`POST /expedientes/{id}/documents` recibe:

```json
{"title":"Documento recibido","stage":"Calificacion de la denuncia.","type":"OTROS","document_date":"2026-10-22","file_url":null}
```

`title`, `stage` y `type` son obligatorios. El titulo admite hasta 255 caracteres. `stage` y `type` se guardan como TEXT, se recortan y no pueden estar vacios. No son IDs, enums ni claves foraneas. Se acepta cualquier texto valido, incluido `OTROS`.

En `stage` se envia el nombre de la etapa general o de una subetapa seleccionada. No es necesario elegir una subetapa. La respuesta devuelve `stage` y `type` como cadenas, sin objeto `substage` ni `substage_id`.

`document_date` es una fecha opcional YYYY-MM-DD y `file_url` es una URL HTTPS opcional. El documento pertenece al expediente y comienza en DRAFT. Devuelve 201, 400 por datos invalidos y 404 si el expediente no existe.

## Selectores del frontend

- `GET /document-stages`: etapas activas, ordenadas, con sus subetapas en `substages`. Cada entrada tiene `id`, `code`, `name` y `sort_order`. El frontend puede seleccionar una etapa general o una subetapa y enviar su `name` en `stage`.
- `GET /document-types`: tipos activos, ordenados, con `id`, `code`, `name` y `sort_order`. Enviar su `name` en `type`.

Son diccionarios de sugerencias. Cambiar o retirar una opcion no altera los documentos guardados. El diccionario inicial de tipos contiene Escrito de oficio, Resolución y OTROS. El de etapas conserva las cinco etapas y quince subetapas, mas OTROS como etapa general. Se pueden cargar mas opciones sin cambiar el contrato.

`GET /expedientes/{id}/documents` devuelve todos los documentos por fecha ascendente (nulos al final), luego created_at e ID. No depende de los diccionarios ni agrupa por etapa.

Los participantes se consultan desde el expediente, con sus endpoints de denunciantes y denunciados. Los documentos no guardan listas de participantes.
