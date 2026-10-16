# Documentos del expediente

Los documentos pertenecen a un expediente y a una subetapa. La etapa principal se obtiene a través de ella. La URL HTTPS es opcional; el binario se sube a R2 mediante el flujo de firma descrito en `docs/uploads.md`.

## Cat?logo de etapas y subetapas

`GET /document-stages` devuelve todas las etapas y subetapas activas. Cada etapa incluye una lista `substages`; ambos niveles est?n ordenados por `sort_order` ascendente (con ID como desempate). Las listas se devuelven vac?as, no `null`, cuando no hay elementos.

```json
[
  {
    "id": 1,
    "code": "CALIFICACION_DENUNCIA",
    "name": "Calificaci?n de la denuncia.",
    "sort_order": 1,
    "substages": [
      {"id": 1, "code": "CALIFICACION", "name": "Calificaci?n de la denuncia.", "sort_order": 1}
    ]
  }
]
```

Usa el `id` de cada subetapa como `substage_id` al crear un documento.

## Crear

`POST /expedientes/{expediente_id}/documents`

```json
{
  "title": "Resolución de calificación",
  "substage_id": 3,
  "document_date": "2026-10-15",
  "file_url": "https://archivos.example.com/documents/1/archivo.pdf"
}
```

`title` es obligatorio (máximo 255 caracteres). `substage_id` identifica una subetapa activa. `document_date` (YYYY-MM-DD) y `file_url` son opcionales; la URL debe ser HTTPS. El estado se asigna `DRAFT`.

Al crear el documento, el servicio busca dentro del expediente a todos los denunciantes y denunciados activos y crea sus relaciones autom?ticamente en la misma transacci?n. El cliente no env?a IDs de participantes. Si el expediente no tiene participantes activos de un tipo, el arreglo correspondiente queda vac?o.

Devuelve `201 Created` con el documento, su subetapa y etapa, y arreglos con los denunciantes y denunciados asociados. `400` indica datos inválidos, subetapa inexistente/inactiva o referencias no válidas. `404` indica expediente inexistente. La operación completa se guarda junta: un error al insertar referencias revierte también el documento.

## Listar por expediente

`GET /expedientes/{expediente_id}/documents`

Devuelve un arreglo vacío si el expediente existe y todavía no tiene documentos. Cada documento incluye subetapa, etapa y referencias individuales. Ordena por `stage.sort_order`, luego `substage.sort_order`, fecha documental ascendente (fechas vacías al final) y finalmente ID.

Un ID de expediente inválido devuelve `400`; si no existe devuelve `404`.

## Ejemplo PowerShell

```powershell
$body = '{"title":"Resolución","substage_id":3,"document_date":"2026-10-15","denunciante_ids":[4],"denunciado_ids":[12]}'
Invoke-RestMethod -Method Post -Uri 'http://localhost:8080/expedientes/1/documents' -ContentType 'application/json' -Body $body
Invoke-RestMethod -Method Get -Uri 'http://localhost:8080/expedientes/1/documents'
```
