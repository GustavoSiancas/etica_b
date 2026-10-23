# Denunciantes y denunciados

Denunciantes y denunciados tienen ID, nombre, tipo y numero de documento. Los denunciados tienen ademas numero de colegiatura. Un mismo registro puede asociarse a varios expedientes; el nombre no es unico.

## Carga inicial y listado

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

Los GET de participantes, las asociaciones al expediente y los remitentes de escritos incluyen estos campos. Los registros anteriores conservan `document_type` y `document_number` en `null`; la colegiatura desconocida se omite. No se inventan documentos para datos existentes.

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
