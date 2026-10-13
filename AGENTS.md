# Guía para continuar el backend BPM

Antes de modificar código, leer en este orden:

1. `specs/PROJECT_CONTEXT.md`
2. `specs/ARCHITECTURE.md`
3. `specs/DEVELOPMENT.md`
4. `specs/SHARED_COMPONENTS.md`
5. `specs/FEATURE_TEMPLATE.md`
6. `specs/FEATURES.md`
7. `specs/IDENTITY.md`
8. `specs/API_ERRORS.md`
9. `specs/OPENAPI.md`
10. `specs/MIGRATIONS.md`
11. `specs/SECURITY.md`
12. `.env.example`

Reglas obligatorias:

- Go 1.27, monolito modular, arquitectura hexagonal y composición manual.
- No usar un framework de inyección de dependencias. Los constructores reciben
  interfaces pequeñas y `internal/core/app` conecta las implementaciones.
- `internal/core` contiene únicamente base técnica compartida; todo concepto de
  negocio se implementa como recorrido vertical en `internal/features`.
- `internal/shared/utils` admite únicamente funciones puras con reutilización
  comprobada; nunca reglas de negocio, configuración o infraestructura.
- `domain` no importa HTTP, pgx, Firebase, R2 ni paquetes de otro adaptador.
- Un handler no contiene reglas de negocio ni abre transacciones.
- Toda consulta empresarial filtra `organization_id` obtenido de una membresía
  autenticada; nunca se confía en el tenant enviado por el cliente.
- Usar UUID/idempotencia y control de versión en operaciones offline.
- Archivos privados: solo metadatos en PostgreSQL y contenido en R2.
- No registrar tokens, contraseñas, URLs firmadas, cuerpos ni cadenas de conexión.
- `migraciones/V1__baseline.sql` es la línea base ya desplegada. Los cambios
  posteriores deben ser archivos nuevos e incrementales.
- Antes de entregar: `gofmt`, `go vet ./...`, `go test ./...`,
  `go test -race ./...` y `git diff --check`.

Estado actual: la plataforma técnica y el primer recorrido de identidad están
construidos. `GET /api/v1/me` resuelve identidad. `features/organizations`
implementa tenant activo, altas de organización/sede y adición de miembros;
leer también `specs/ORGANIZATIONS.md`. No confundir el catálogo del workspace
con ejecución de capacitación, checklists o incidencias: esos recorridos faltan.

