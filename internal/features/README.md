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

Están implementados `identity`, `organizations` y la primera versión vertical
de `checklists`. El alcance y los siguientes módulos se documentan en
`specs/FEATURES.md`; la plantilla de implementación está en
`specs/FEATURE_TEMPLATE.md`.

