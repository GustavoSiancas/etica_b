# Denunciantes y denunciados

Para la ficha individual, `GET /denunciantes/{id}/expedientes` y `GET /denunciados/{id}/expedientes` devuelven sus expedientes paginados, con filtros `status` y `location`. Ver [contrato y ejemplos para ambos roles](expedientes-por-participante.md).

Denunciantes y denunciados tienen ID, nombre, tipo y numero de documento. Los denunciados tienen ademas numero de colegiatura. Un mismo registro puede asociarse a varios expedientes; el nombre no es unico.

## Carga inicial y listado

La búsqueda por nombre en `GET /denunciantes` y `GET /denunciados`, tanto con `name` como con `q`, ignora mayúsculas, tildes y la diferencia entre `ñ` y `n`. Por ejemplo, `?name=Jose%20Munoz` encuentra `José Muñoz`, y una búsqueda con tildes también encuentra nombres guardados sin ellas. Se conserva el nombre original en la respuesta y se mantiene la paginación. No requiere migración ni extensiones de PostgreSQL.

El arranque carga automáticamente `seeds/demo_data.sql` con los colegiados del CSV como denunciados. El importador `scripts/import_participants.py` mapea `numero_de_cal` a `registration_number`, `nombre_completo` a `name`, `tipo_documento` (DNI/CE) a `document_type` y `numero_documento` a texto en `document_number`. Solo inserta CAL faltantes; no borra ni modifica registros existentes ni crea otros datos de prueba. No depende de las banderas para habilitar POST o recarga de desarrollo.

`POST /denunciantes` y `POST /denunciados` estan desactivados por defecto (404). Solo se habilitan con `ENABLE_INITIAL_PARTICIPANT_LOAD=true` durante la carga inicial. Al terminar, volver a false y reiniciar la aplicacion. La bandera no modifica los datos ni afecta los GET. El seed SQL sigue siendo otra forma de carga inicial.

`POST /denunciantes` recibe todos estos campos obligatorios:

```json
{"name":"Maria Perez","document_type":"DNI","document_number":"00123456"}
```

`document_type` permite `DNI`, `RUC` o `CE`.

`POST /denunciados` recibe todos estos campos obligatorios:

```json
{"name":"Luis Perez","document_type":"CE","document_number":"000123456","registration_number":12345}
```

Para denunciados, `document_type` permite solo `DNI` o `CE`; `registration_number` es un entero positivo (colegiatura). El numero de documento se envia como texto de 1 a 20 digitos para conservar ceros iniciales. El servicio recorta espacios y normaliza el tipo a mayusculas. No valida contra un padron externo ni impone unicidad del documento o colegiatura.

Los GET de participantes y las asociaciones al expediente incluyen estos campos. Los registros anteriores conservan `document_type` y `document_number` en `null`; la colegiatura desconocida se omite. No se inventan documentos para datos existentes.

`GET /denunciantes?limit=20&offset=0` y `GET /denunciados?limit=20&offset=0` devuelven una pagina con `data`, `total`, `limit` y `offset`.

## Asociar al crear el expediente

Enviar los IDs existentes en `POST /expedientes`:

```json
{
  "description":"Descripcion del caso",
  "opening_date":"2026-10-22",
  "init_type":"DENUNCIA_DE_PARTE",
  "denunciante_ids":[1,2],
  "denunciado_ids":[3]
}
```

El expediente y todas sus asociaciones se guardan en una sola transaccion. Si un ID no existe, devuelve 404 y no se crea el expediente. IDs no positivos o repetidos dentro de una lista devuelven 400. Las listas pueden omitirse o enviarse vacias cuando no corresponda; un mismo ID numerico puede aparecer en ambos roles porque son tablas distintas.

Las asociaciones son permanentes, sin estado, fecha de ingreso ni historial. Las rutas POST para agregar participantes a un expediente existente ya no estan disponibles. Los participantes del expediente se definen durante su creacion.

## Listar participantes del expediente

`GET /expedientes/{id}/denunciantes` y `GET /expedientes/{id}/denunciados` devuelven todas las asociaciones permanentes del expediente, ordenadas por ID de asociaci?n ascendente, sin paginaci?n.

```json
[
  {"id":5,"expediente_id":1,"participant":{"id":2,"name":"Ana","document_type":"DNI","document_number":"00123456"}}
]
```

`id` identifica la asociacion y `participant.id` identifica la persona. Devuelven `200` y `[]` si el expediente existe sin participantes, `400` para un ID invalido y `404` si el expediente no existe.

## Filtros y paginacion de listados generales

`GET /denunciantes` y `GET /denunciados` aceptan:

| Parametro | Comportamiento |
| --- | --- |
| limit | Cantidad por pagina, 1 a 100. Predeterminado: 20. |
| offset | Registros a omitir, desde 0. Predeterminado: 0. |
| q | Coincidencia parcial por nombre o numero de documento; en denunciados incluye colegiatura. |
| name | Coincidencia parcial por nombre, sin distinguir mayusculas. |
| document_type | Tipo exacto: DNI/RUC/CE para denunciantes, DNI/CE para denunciados. |
| document_number | Numero exacto, como texto de 1 a 20 digitos. |
| registration_number | Solo denunciados: colegiatura exacta, entero positivo. |

Los filtros son opcionales, se recortan y se combinan con AND. La busqueda por nombre conserva la distincion de acentos. `q` busca entre sus campos con OR. No repetir un parametro de filtro. Los filtros invalidos devuelven 400.

Ejemplos:

```http
GET /denunciantes?limit=20&offset=0&name=Maria&document_type=DNI
GET /denunciantes?document_number=00123456
GET /denunciados?limit=10&offset=10&q=Garcia
GET /denunciados?registration_number=12345
```

Respuesta: `{"data":[],"total":0,"limit":20,"offset":0}`. `total` cuenta las coincidencias antes de paginar. El orden es ID ascendente; sin coincidencias, data es un arreglo vacio. Los GET por expediente conservan su listado completo de asociaciones sin paginacion.
