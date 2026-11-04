# ERD de expedientes y documentos

Esquema actual: migracion inicial y mejoras 002 a 008, aplicadas automaticamente al iniciar. La 008 mueve el estado y la ubicacion a las carpetas.

## Expedientes, secciones y documentos

```mermaid
erDiagram
    expedientes ||--o{ expediente_folders : contiene
    folder_types ||--o{ expediente_folders : clasifica
    expediente_folders ||--o{ expediente_section : contiene
    expediente_section o|--o{ expediente_section : tiene_subsecciones
    expediente_section ||--o{ expediente_section_documents : agrupa
    documents ||--o{ expediente_section_documents : vincula

    expedientes {
        bigint id PK
        text description
        date opening_date "Opcional"
        varchar number UK
        expediente_init_type_enum init_type "Opcional"
        timestamptz created_at
        timestamptz updated_at
    }

    folder_types {
        bigint id PK
        text name UK
        varchar abbreviation "EP o MC para los tipos iniciales"
    }

    expediente_folders {
        bigint id PK
        bigint expediente_id FK
        bigint folder_type_id FK
        timestamptz opening_date
        varchar status "ABIERTO o CERRADO"
        varchar location "DIRECCION DE ETICA por defecto"
        timestamptz created_at
        timestamptz updated_at
    }

    expediente_section {
        bigint id PK
        bigint expediente_id FK
        bigint expediente_folder_id FK
        bigint expediente_section_parent_id FK "Opcional; padre del mismo expediente"
        text name "Nombre obligatorio"
        text type "Tipo obligatorio, texto libre"
        date document_date "Fecha opcional"
        integer sort_order "Orden desde cero"
        varchar color_hex "#RRGGBB; defecto #808080"
        timestamptz created_at
        timestamptz updated_at
    }

    expediente_section_documents {
        bigint expediente_section_id PK, FK
        bigint document_id PK, FK
    }

    documents {
        bigint id PK
        varchar name "Nombre obligatorio; maximo 255"
        text file_url "Enlace opcional"
        timestamptz created_at
        timestamptz updated_at
    }
```

- Cada sección pertenece a un único expediente. Un expediente puede tener cero o varias secciones.
- Una sección puede contener varios documentos. La tabla intermedia impide repetir el mismo documento dentro de la misma sección.
- Un documento puede estar en cero o varias secciones, incluso de distintos expedientes: el esquema actual permite esa reutilización.
- El documento se relaciona con el expediente a través de la sección; no tiene `expediente_id` propio.
- Nombre, tipo, fecha, orden y color pertenecen a la sección.
- El estado y la ubicacion pertenecen a cada carpeta. La migracion 008 copia los valores historicos a EXPEDIENTE PRINCIPAL y elimina las columnas del expediente.
- Una carpeta cerrada protege su contenido; un documento compartido se bloquea si alguna de sus carpetas esta cerrada.
- `expediente_section_parent_id = NULL` identifica una raíz. Cada sección puede tener hijas sin límite fijo de profundidad, siempre dentro del mismo expediente y sin ciclos.
- `sort_order` ordena entre hermanas; los empates se resuelven por ID. Para borrar una sección con hijas primero deben moverse o eliminarse sus hijas.
- Eliminar una sección elimina sus vínculos y conserva los documentos. Para eliminar un documento primero deben retirarse sus vínculos.

## Participantes del expediente

```mermaid
erDiagram
    expedientes ||--o{ expediente_denunciantes : tiene
    denunciantes ||--o{ expediente_denunciantes : participa
    expedientes ||--o{ expediente_denunciados : tiene
    denunciados ||--o{ expediente_denunciados : participa

    expedientes {
        bigint id PK
        varchar number UK
    }

    expediente_denunciantes {
        bigint id PK
        bigint expediente_id FK "Unico junto con denunciante_id"
        bigint denunciante_id FK
    }

    denunciantes {
        bigint id PK
        varchar name
        varchar document_type "Opcional: DNI, RUC o CE"
        varchar document_number "Opcional"
        boolean is_demo
        timestamptz created_at
        timestamptz updated_at
    }

    expediente_denunciados {
        bigint id PK
        bigint expediente_id FK "Unico junto con denunciado_id"
        bigint denunciado_id FK
    }

    denunciados {
        bigint id PK
        varchar name
        varchar document_type "Opcional: DNI o CE"
        varchar document_number "Opcional"
        bigint registration_number "Opcional; mayor que cero"
        boolean is_demo
        timestamptz created_at
        timestamptz updated_at
    }
```

## Catalogos y contador conservados

Estos objetos permanecen en la base. `expediente_section.type` es texto libre y no tiene FK hacia `document_types`. Este catálogo ofrece sugerencias de tipo: Escrito de oficio, Resolución y OTROS. El endpoint `GET /document-types` lista las opciones activas por orden.

`document_stages` y `document_substages` no se crean en el esquema consolidado; las secciones reemplazan la organización anterior por etapas.

```mermaid
erDiagram
    document_types {
        bigint id PK
        varchar code UK
        varchar name
        integer sort_order
        boolean is_active
    }

    expediente_year_counters {
        integer year PK "Entre 1 y 9999"
        bigint last_number "Mayor que cero"
    }

    app_schema_migrations {
        varchar version PK "Registro interno de migraciones aplicadas"
        timestamptz applied_at
    }
```

El contador anual asigna `expedientes.number` mediante un trigger usando el año de `opening_date`; no existe una FK entre ambas tablas. La migración 003 actualiza este comportamiento en bases existentes y se ejecuta automáticamente al arrancar, conservando los números previos.

Las tablas `escritos`, `escrito_denunciantes` y `escrito_denunciados` no se crean en el esquema consolidado.
