# Features

Cada carpeta representa una capacidad del negocio y contiene su recorrido
vertical completo. Se crea una feature cuando existe un caso de uso real; no se
crean carpetas vacías por tabla.

```text
<feature>/
  domain/       entidades, valores, errores y reglas puras
  application/  casos de uso y puertos requeridos por esos casos de uso
  adapters/
    postgres/   implementación de persistencia
    http/       handlers, DTO, validación de entrada y rutas
```

Reglas:

- `domain` no depende de `core`, HTTP, PostgreSQL ni SDK externos.
- `application` coordina reglas y transacciones mediante interfaces pequeñas.
- `adapters` traduce protocolos y tecnología; no decide reglas del negocio.
- Una feature no importa los adaptadores de otra. Si comparten una regla real,
  se define un contrato explícito o se extrae un concepto con nombre de dominio.
- Los helpers permanecen privados y cerca de su uso. No existe una carpeta
  genérica `utils`.

La primera feature será `identity`; el orden restante está en
`specs/FEATURES.md` y la plantilla de implementación en
`specs/FEATURE_TEMPLATE.md`.

