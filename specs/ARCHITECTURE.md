# Arquitectura del backend BPM

## Decisión

El backend es un monolito modular en Go con arquitectura hexagonal, Clean
Architecture y separación por features. Hay dos zonas: `core` para la base
técnica compartida y `features` para capacidades del negocio.

```text
cmd/api/                         punto de entrada mínimo
internal/core/
  app/                           ciclo de vida y composición de dependencias
  config/                        configuración tipada y validada
  contracts/                     contratos técnicos transversales
  database/postgres/             pool y configuración común de PostgreSQL
  httpserver/                    servidor, router, health y middleware
  integrations/firebase/         identidad externa
  integrations/r2/               almacenamiento privado
  observability/                 logs y reporte de errores
internal/features/<feature>/     dominio, casos de uso y adaptadores
internal/shared/                 primitivas reutilizables y utils gobernados
migraciones/V1__baseline.sql     línea base ya instalada
```

`core` no es una capa de negocio ni un lugar para código sobrante. Contiene la
infraestructura compartida, la configuración y el ensamblaje del proceso. Todo
concepto funcional vive dentro de una feature.

## Forma de una feature

Se agrega una carpeta únicamente al implementar un recorrido real:

```text
internal/features/checklists/
  domain/          entidades, valores, errores y reglas puras
  application/     casos de uso y puertos requeridos
  adapters/
    postgres/      consultas y repositorios concretos
    http/           handlers, DTO, validación y registro de rutas
```

El paquete raíz de la feature puede exponer un constructor pequeño para montar
sus rutas y dependencias desde `core/app/container.go`. No expone detalles de
sus adaptadores.

## Dirección de dependencias

```text
cmd/api -> core/app -> core + features/adapters
                              |
                              v
                     features/application
                              |
                              v
                       features/domain
```

Reglas:

1. `domain` no importa `core`, HTTP, pgx, Firebase, R2 ni otros adaptadores.
2. `application` depende de su dominio y de interfaces declaradas cerca del caso
   de uso que las consume.
3. Los adaptadores implementan esas interfaces y traducen HTTP, SQL o SDK.
4. Las transacciones empiezan y terminan en los casos de uso.
5. Los handlers validan formato, invocan un caso de uso y traducen su resultado;
   no contienen reglas ni abren transacciones.
6. Una feature no importa directamente adaptadores de otra feature.
7. El tenant se deriva de la identidad y membresía verificadas; nunca se confía
   solo en un `organization_id` recibido del cliente.
8. PostgreSQL es la fuente transaccional. Firebase aporta identidad/FCM y R2
   conserva objetos privados.
9. Los efectos externos se registran en `outbox_events` en la transacción y se
   procesan posteriormente.

## Inyección de dependencias

Se usa inyección por constructores y composición manual. No se utiliza un
framework de DI.

- `internal/core/app/container.go` es la única raíz de composición.
- El contenedor crea pool, proveedores externos, casos de uso, handlers y
  servidor en ese orden.
- Un caso de uso recibe solo los puertos que necesita.
- Un handler recibe casos de uso, nunca el pool ni clientes externos.
- Las pruebas inyectan dobles simples que implementan las mismas interfaces.
- No hay singletons mutables ni localizadores de servicios.

Ejemplo esperado dentro de una feature:

```go
type Repository interface {
    FindByID(ctx context.Context, organizationID, id string) (Entity, error)
}

type GetEntity struct {
    repository Repository
}

func NewGetEntity(repository Repository) *GetEntity {
    return &GetEntity{repository: repository}
}
```

Las interfaces nacen donde se consumen. `core/contracts` se reserva para
capacidades técnicas usadas por varias features, como verificación de tokens y
almacenamiento privado.

## Utilidades y código compartido

La primera opción es ubicar una función junto al concepto que representa:

- request ID y autenticación HTTP: `core/httpserver/middleware`;
- codificación de respuestas: `core/httpserver/response`;
- validación de claves R2: `core/integrations/r2`;
- normalización propia de un módulo: dentro de esa feature.

`internal/shared/utils` está reservado para funciones puras reutilizadas por dos
o más consumidores. No admite reglas de negocio ni dependencias de
infraestructura. La política completa está en `SHARED_COMPONENTS.md`.

## Convenciones de código limpio

- Paquetes pequeños, cohesionados y con nombres del negocio o la capacidad.
- Constructores validan dependencias obligatorias cuando puedan faltar.
- Errores de dominio comparables; los handlers los traducen a HTTP.
- DTO HTTP, entidades y filas PostgreSQL son tipos distintos.
- No hay repositorio genérico.
- Funciones cortas; comentarios explican decisiones, no repiten el código.
- `context.Context` es el primer argumento y no se guarda en structs.
- Fechas en UTC; la zona de sede se aplica al presentar o agrupar.
- Ninguna goroutine nace sin dueño, cancelación y política de cierre.

## Contratos disponibles

- `contracts.TokenVerifier`: validación revocable de tokens Firebase.
- `contracts.ObjectStorage`: carga, descarga y verificación de evidencia privada.
- Pool PostgreSQL limitado y configurado en UTC.
- `/health`: vida del proceso.
- `/ready`: disponibilidad de PostgreSQL.
- `/openapi.yaml` y `/docs/`: contrato OpenAPI y Swagger UI autocontenida.
- Middleware de request ID, recuperación, cabeceras, CORS, límite de cuerpo,
  tiempo máximo, rate limit y logs sin query strings ni cuerpos.
- Contrato HTTP de errores sanitizados con arreglo `errors`, códigos estables,
  mensajes seguros, `fields` para formularios y `requestId`.

El contexto funcional está en `PROJECT_CONTEXT.md` y el alcance posterior en
`FEATURES.md`.
