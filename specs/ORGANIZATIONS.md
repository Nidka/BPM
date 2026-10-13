# Organizaciones y espacio de trabajo

## Implementado

`features/organizations` es el segundo recorrido vertical: HTTP → aplicación →
puerto UnitOfWork/Store → PostgreSQL. La composición manual está en `module.go`
y `core/app/container.go`. No importa adaptadores de identidad: recibe el UID
del middleware y vuelve a validar usuario y permisos en PostgreSQL.

| Ruta bajo `/api/v1` | Acceso |
|---|---|
| GET /organizations | Organizaciones activas con membresía activa; SUPER_ADMIN ve todas |
| POST /organizations | Solo SUPER_ADMIN |
| GET /organizations/{id}/workspace | Miembro activo o SUPER_ADMIN |
| POST /organizations/{id}/locations | ADMIN del tenant o SUPER_ADMIN |
| POST /organizations/{id}/members | ADMIN del tenant o SUPER_ADMIN |

Todas requieren token Firebase válido, correo verificado y cuenta BPM activa.
El tenant de la URL es una solicitud de acceso, no una autorización. Un ADMIN de
una organización no administra otra. CHEF/WORKER solo ven sus sedes asignadas;
la lista de personas solo se devuelve a administradores. Los trabajadores ven
únicamente incidencias que reportaron en sus sedes autorizadas.

El workspace incluye catálogo de cursos publicados con el progreso del usuario,
plantillas activas e incidencias visibles. Son consultas de base, no ejecución de
los módulos: no hay lecciones, evaluaciones, respuestas ni cambios de estado.
Cada lista limita a 100 y las descripciones se resumen a 500 caracteres; no
representan un total global. La paginación es el
próximo incremento antes de crecer por encima de ese límite.

## Altas, auditoría y errores

- Organización: razón social, nombre comercial, tipo, identificación fiscal y
  contacto opcional. Un identificador fiscal vacío se guarda NULL, no como un
  valor único compartido por todas las empresas.
- Sede: nombre, dirección, distrito/teléfono opcionales; zona America/Lima.
- Miembro: cuenta BPM activa existente por correo, rol ADMIN/CHEF/WORKER y
  sedes del mismo tenant. CHEF/WORKER requieren al menos una. ADMIN accede a todas.
  No es una invitación: no crea cuentas Firebase ni envía correo.
- Las operaciones y `audit_events` se confirman en una transacción. created_by
  y updated_by salen del actor autenticado; las fechas las genera PostgreSQL.
- UUID cliente estable para organización/sede: reintento idéntico devuelve 200
  sin duplicar ni reescribir datos/auditoría. Mismo ID con otro contenido: 409.
  Membresía existente: 409, nunca cambia su rol silenciosamente.
- JSON hasta 16 KiB, campos desconocidos rechazados. errors[] usa code/message/
  fields; no se exponen SQL, tokens, cadenas de conexión ni excepciones.

## Puesta en marcha y límites

Se usa V1 existente sin modificarla ni reaplicarla. No hace falta una migración
nueva para estas altas. Nunca ejecutar la baseline contra la base con datos.

El primer SUPER_ADMIN debe designarlo el responsable de la plataforma mediante
un procedimiento administrativo autorizado y auditado. Ni el registro público
ni este cambio convierten automáticamente cuentas USER en administradores.
Luego: crear organización → crear sede → agregar ADMIN existente → incorporar
colaboradores a sus sedes. No hay endpoint público de elevación de plataforma.

Pendientes: edición/desactivación/transferencia de administración, invitaciones,
paginación, dispositivos, sincronización offline, ejecución de controles,
multimedia R2, lecciones/evaluaciones, operación diaria, alertas e investigación.
Los UUID de alta facilitan reintentos; no equivalen a una cola offline.

## Verificación

Unitarias: validación sin acceso a DB. HTTP: token, correo no verificado,
campos desconocidos, errores por campo y sanitización. Integración PostgreSQL:
baseline real en esquema aislado de `bpm_test`, múltiples tenants, roles,
usuario/membresía inactivos, sedes ajenas, repetición, auditoría y catálogo.
`TEST_DATABASE_URL` es solo de pruebas; no usar las credenciales de Railway.

Contrato completo: `internal/core/httpserver/docs/openapi.yaml`.
