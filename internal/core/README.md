# Core

`core` es el núcleo técnico compartido del proceso. No contiene reglas de
restaurantes, checklists, incidencias ni academia.

```text
core/
  app/             arranque, cierre y composición manual de dependencias
  config/          lectura y validación del entorno
  contracts/       contratos técnicos compartidos por varias features
  database/        conexión y configuración común de PostgreSQL
  httpserver/      servidor, router, health, middleware y respuestas base
  integrations/    adaptadores de proveedores externos (Firebase y R2)
  observability/   logs, banner local de arranque y reporte de errores
```

`app.Container` es la única raíz de composición. Construye implementaciones
concretas y las inyecta mediante constructores. No hay variables globales,
service locator ni framework de DI.

El servidor recibe el contrato `contracts.StartupReporter`. Su implementación
visual vive en `observability` y se muestra solo cuando la base de datos está
conectada y el puerto HTTP fue reservado correctamente.

No se crea `core/utils`: una función técnica vive en el paquete del concepto que
representa. Las funciones puras con reutilización comprobada pueden promoverse
a `internal/shared/utils` siguiendo `specs/SHARED_COMPONENTS.md`.
