# Plantilla de una feature

Esta es la estructura de referencia. Se crean únicamente los archivos que el
caso de uso necesite.

```text
internal/features/<nombre>/
  domain/
    entity.go          entidad y comportamiento
    errors.go          errores comparables del dominio
  application/
    ports.go           interfaces requeridas por el caso de uso
    create.go          un caso de uso por responsabilidad
  adapters/
    postgres/
      repository.go    implementación SQL del puerto
    http/
      handler.go       DTO, validación de formato y traducción de errores
      routes.go        registro de rutas
  module.go            ensamblaje público de la feature
```

## Ensamblaje

`module.go` recibe dependencias técnicas ya construidas y devuelve la entrada
HTTP de la feature. `core/app/container.go` es el único lugar que conoce y
conecta todas las implementaciones concretas.

```go
type Dependencies struct {
    Database *pgxpool.Pool
}

func New(deps Dependencies) (*Module, error) {
    repository := postgres.NewRepository(deps.Database)
    useCase := application.NewCreate(repository)
    return &Module{Handler: httpadapter.NewHandler(useCase)}, nil
}
```

El ejemplo muestra la forma, no una obligación de exponer `pgxpool` a capas
internas. Solo el adaptador PostgreSQL lo recibe; el caso de uso conoce su
interfaz `Repository`.

## Responsabilidades

- Dominio: invariantes y transiciones, sin etiquetas JSON ni SQL.
- Aplicación: autorización, coordinación, idempotencia y transacción.
- PostgreSQL: consultas, mapeo de filas y errores del driver.
- HTTP: autenticación técnica, DTO, códigos de estado y serialización.
- `module.go`: construcción; no contiene lógica de negocio.

## Compartir sin acoplar

Primero mantener un helper privado dentro de la feature. Si una segunda feature
necesita exactamente el mismo concepto, aplicar la política de
`SHARED_COMPONENTS.md`. Nunca importar `adapters` de otra feature.

## Pruebas mínimas

1. Dominio: reglas y transiciones.
2. Caso de uso: permisos, tenant, errores e idempotencia con dobles.
3. Repositorio: PostgreSQL real y aislamiento por `organization_id`.
4. Handler: contrato HTTP, validación y no filtración de errores internos.
