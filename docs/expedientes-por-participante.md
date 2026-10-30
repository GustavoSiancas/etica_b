# Expedientes por denunciante y denunciado

Estos endpoints sirven para mostrar los expedientes asociados en la ficha de un participante. Ambos devuelven el mismo contrato paginado y permiten combinar filtros de estado y ubicación. Incluyen expedientes abiertos y cerrados; son operaciones de consulta.

## 1. Expedientes de un denunciante

```http
GET /denunciantes/{id}/expedientes
```

Ejemplos:

```http
GET /denunciantes/12/expedientes?limit=20&offset=0
GET /denunciantes/12/expedientes?status=ABIERTO&limit=20&offset=0
GET /denunciantes/12/expedientes?status=CERRADO&limit=20&offset=0
GET /denunciantes/12/expedientes?location=CONSEJO%20DE%20ETICA&limit=20&offset=0
GET /denunciantes/12/expedientes?status=ABIERTO&location=DIRECCION%20DE%20ETICA&limit=10&offset=10
```

El ID corresponde a `denunciantes.id`. Solo se devuelven expedientes relacionados mediante `expediente_denunciantes`.

## 2. Expedientes de un denunciado

```http
GET /denunciados/{id}/expedientes
```

Ejemplos:

```http
GET /denunciados/35/expedientes?limit=20&offset=0
GET /denunciados/35/expedientes?status=ABIERTO&limit=20&offset=0
GET /denunciados/35/expedientes?status=CERRADO&limit=20&offset=0
GET /denunciados/35/expedientes?location=TRIBUNAL%20DE%20HONOR&limit=20&offset=0
GET /denunciados/35/expedientes?status=CERRADO&location=CONSEJO%20DE%20ETICA&limit=10&offset=0
```

El ID corresponde a `denunciados.id`. Solo se devuelven expedientes relacionados mediante `expediente_denunciados`.

**En ambas rutas se usa el ID interno del registro, nunca el DNI, RUC, CE ni el número de CAL.** Se obtiene del campo `id` en los listados `GET /denunciantes` o `GET /denunciados`. Una persona que figure en ambos registros tiene un ID independiente para cada rol.

## 3. Parámetros comunes

| Parámetro | Tipo | Predeterminado | Valores |
| --- | --- | --- | --- |
| `id` | Entero positivo, en la ruta | Obligatorio | ID interno del participante del rol indicado |
| `limit` | Entero | `20` | De `1` a `100` |
| `offset` | Entero | `0` | Desde `0` |
| `status` | Texto opcional | Todos los estados | `ABIERTO`, `CERRADO` |
| `location` | Texto opcional | Todas las ubicaciones | `DIRECCION DE ETICA`, `CONSEJO DE ETICA`, `TRIBUNAL DE HONOR` |
| `from` | Fecha opcional | Sin límite inferior | `YYYY-MM-DD`; fecha de apertura mínima, inclusiva |
| `to` | Fecha opcional | Sin límite superior | `YYYY-MM-DD`; fecha de apertura máxima, inclusiva |

- Los filtros son de igualdad y se combinan con AND.
- Sin filtros se listan todos los expedientes del participante, incluidos los cerrados.
- `total` cuenta únicamente sus expedientes que cumplen los filtros, antes de aplicar la paginación.
- Orden: `created_at DESC, id DESC` (más recientes primero; ID como desempate).
- No se repiten expedientes dentro del listado.
- Para la segunda página de 20 registros, usar `limit=20&offset=20`; para la tercera, `offset=40`.
- Al cambiar un filtro, reiniciar `offset=0`.
- Codificar los espacios de la ubicación como `%20` o usar `URLSearchParams`.
- `from` y `to` filtran `opening_date`, no la fecha de creación. Pueden enviarse juntos o por separado. Con ambos límites debe cumplirse `from <= to`; fechas inválidas o repetidas devuelven 400. Un rango de un solo día es válido. Los expedientes sin fecha de apertura no coinciden cuando se aplica un límite de fecha.

Ejemplos con rango, disponibles también en el listado general:

```http
GET /expedientes?from=2026-10-01&to=2026-10-31&limit=20&offset=0
GET /denunciantes/12/expedientes?from=2026-10-01&to=2026-10-31&status=ABIERTO&limit=20&offset=0
GET /denunciados/35/expedientes?from=2026-10-01&to=2026-10-31&location=CONSEJO%20DE%20ETICA&limit=20&offset=0
```

## 4. Respuesta común — 200 OK

Los valores del siguiente ejemplo son ilustrativos:

```json
{
  "data": [
    {
      "id": 101,
      "description": "Descripción del expediente",
      "opening_date": "2026-10-30",
      "number": "1-2026",
      "init_type": "DENUNCIA_DE_PARTE",
      "status": "ABIERTO",
      "location": "DIRECCION DE ETICA",
      "created_at": "2026-10-30T15:00:00Z",
      "updated_at": "2026-10-30T15:00:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

Cada elemento contiene los mismos campos que `GET /expedientes`. `opening_date` e `init_type` pueden ser `null` en registros anteriores. No incluye secciones, documentos ni otros participantes; se consultan mediante las rutas existentes del expediente.

Si el participante existe pero no tiene expedientes, los filtros no encuentran coincidencias o el offset excede los resultados, la respuesta es 200 con `data: []`. Un offset fuera de rango conserva el total real filtrado:

```json
{"data":[],"total":3,"limit":20,"offset":20}
```

## 5. Errores

| Estado HTTP | Motivo |
| --- | --- |
| `400` | ID inválido, paginación inválida, estado/ubicación desconocidos o parámetros reconocidos repetidos |
| `404` | No existe el participante en el rol consultado |
| `500` | Error interno; no expone detalles SQL |

Ejemplo de participante inexistente:

```json
{"error":"participant not found"}
```

El estado CERRADO no bloquea estas consultas. No necesitan habilitar `ENABLE_INITIAL_PARTICIPANT_LOAD` ni crear una migración.

## 6. Ejemplo para frontend

```typescript
type Role = 'denunciantes' | 'denunciados';
type Status = 'ABIERTO' | 'CERRADO';
type Location = 'DIRECCION DE ETICA' | 'CONSEJO DE ETICA' | 'TRIBUNAL DE HONOR';

async function getParticipantExpedientes(
  role: Role,
  participantId: number,
  { limit = 20, offset = 0, status, location, from, to }: {
    limit?: number;
    offset?: number;
    status?: Status;
    location?: Location;
    from?: string; // YYYY-MM-DD
    to?: string;   // YYYY-MM-DD
  } = {}
) {
  const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (status) query.set('status', status);
  if (location) query.set('location', location);
  if (from) query.set('from', from);
  if (to) query.set('to', to);

  const response = await fetch(
    `http://localhost:8080/${role}/${participantId}/expedientes?${query}`
  );
  if (!response.ok) {
    const body = await response.json();
    throw new Error(body.error ?? 'No se pudieron consultar los expedientes');
  }
  return response.json(); // { data, total, limit, offset }
}

// Ficha de denunciante:
await getParticipantExpedientes('denunciantes', 12, { status: 'ABIERTO' });

// Ficha de denunciado:
await getParticipantExpedientes('denunciados', 35, {
  status: 'CERRADO',
  location: 'TRIBUNAL DE HONOR',
  limit: 10,
  offset: 0
});
```
