# Subida directa de imágenes a R2

No hay autenticación ni usuarios. El documento debe existir en `documents` antes de solicitar una firma. Aplicar las migraciones y seeds según el README; estos endpoints no crean documentos.

## Configuración

Completar `.env`:

```dotenv
HTTP_ADDR=:8080
R2_PUBLIC_URL=https://tu-dominio-publico.example.com
```

`R2_PUBLIC_URL` es el origen público del bucket, sin rutas: dominio personalizado o URL pública r2.dev habilitada en Cloudflare. No es el endpoint S3 `R2_ENDPOINT`. La aplicación no configura el dominio ni publica el bucket automáticamente. La URL pública permite acceder a las imágenes sin firma; la URL firmada temporal solo autoriza la subida. Ver [URLs firmadas de Cloudflare](https://developers.cloudflare.com/r2/api/s3/presigned-urls/).

Para permitir cargas desde cualquier dominio, en las opciones CORS del bucket R2 permitir todos los or?genes:

```json
[
  {
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["PUT", "GET", "HEAD"],
    "AllowedHeaders": ["Content-Type"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3600
  }
]
```

Referencia: [CORS de R2](https://developers.cloudflare.com/r2/buckets/cors/).

Ejecutar `go run ./cmd/app`. Ahora permanece escuchando HTTP; Ctrl+C inicia su cierre.

## Flujo del frontend

1. `POST /documents/123/upload-url` con `{"content_type":"image/png"}`. Devuelve `upload_url`, `file_url`, `method`, `headers`, `expires_in` (300 segundos) y `max_size_bytes` (10 MiB). Admite PNG, JPEG, WebP y GIF. La clave se genera con aleatoriedad criptográfica bajo `documents/123/`.
2. Subir el archivo binario directamente a `upload_url` mediante PUT, usando exactamente los headers devueltos. No usar multipart ni enviar el archivo al backend.
3. `PATCH /documents/123/file-url` con `{"file_url":"<file_url recibido>"}`. El backend verifica pertenencia al prefijo del documento, existencia, Content-Type declarado y tamaño del objeto mediante HeadObject. Solo entonces actualiza `documents.file_url` y `updated_at`. No guarda la URL firmada ni el binario.

```javascript
async function uploadImage(documentId, file) {
  const api = 'http://localhost:8080';
  const signatureResponse = await fetch(`${api}/documents/${documentId}/upload-url`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content_type: file.type })
  });
  if (!signatureResponse.ok) throw new Error('No se pudo generar la firma');
  const signature = await signatureResponse.json();
  if (file.size > signature.max_size_bytes) throw new Error('Imagen demasiado grande');
  const uploaded = await fetch(signature.upload_url, {
    method: signature.method, headers: signature.headers, body: file
  });
  if (!uploaded.ok) throw new Error('No se pudo subir a R2');
  const saved = await fetch(`${api}/documents/${documentId}/file-url`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ file_url: signature.file_url })
  });
  if (!saved.ok) throw new Error('No se pudo guardar la URL');
  return (await saved.json()).file_url;
}
```

Errores: 400 petición inválida, 404 documento inexistente, 502 objeto no verificable en R2, 500 error interno. Las respuestas no contienen credenciales ni errores SQL.

## Límites del flujo

La firma puede reutilizarse hasta que expire; no es de un solo uso. PUT no impone un límite duro de tamaño: se comprueba al confirmar, por lo que un objeto rechazado puede quedar en R2. La validación de tipo usa metadatos, no inspecciona el contenido binario. Una subida abandonada o un reemplazo deja el objeto anterior en R2; no se implementa limpieza automática. CORS permite el navegador configurado, pero no constituye autenticación. La confirmación guarda exclusivamente URLs del dominio y prefijo esperado; no admite URLs externas.

Las pruebas automatizadas verifican validaciones de URLs, tipos, JSON y CORS. La subida real requiere completar el dominio público, configurar CORS p?blico y disponer de un documento existente; no se verifica automáticamente ni se altera Cloudflare durante las pruebas.
