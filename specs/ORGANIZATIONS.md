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
| GET /organizations/{id}/locations | Sedes activas autorizadas, paginadas |
| GET /organizations/{id}/members | ADMIN del tenant/SUPER_ADMIN, paginado |
| GET /organizations/{id}/courses | Cursos publicados y progreso propio, paginado |
| GET /organizations/{id}/checklist-summaries | Resúmenes de plantillas activas, paginados |
| GET /organizations/{id}/incidents | Incidencias de sedes autorizadas, paginadas |
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
Las descripciones se resumen a 500 caracteres. Workspace devuelve la primera
página, `nextPageTokens` independientes y `totals` de sedes permitidas, cursos
publicados y plantillas publicadas. Los indicadores cuentan el conjunto autorizado
completo y no dependen de cuántos registros cargó el cliente.

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
dispositivos, sincronización offline, ejecución de controles,
multimedia R2, lecciones/evaluaciones, operación diaria, alertas e investigación.
Los UUID de alta facilitan reintentos; no equivalen a una cola offline.

## Verificación

Unitarias: validación sin acceso a DB. HTTP: token, correo no verificado,
campos desconocidos, errores por campo y sanitización. Integración PostgreSQL:
baseline real en esquema aislado de `bpm_test`, múltiples tenants, roles,
usuario/membresía inactivos, sedes ajenas, repetición, auditoría y catálogo.
`TEST_DATABASE_URL` es solo de pruebas; no usar las credenciales de Railway.

Contrato completo: `internal/core/httpserver/docs/openapi.yaml`.

## Paginación de lectura

`GET /organizations` y las listas de la tabla aceptan exclusivamente `pageSize`
(1–100), `pageToken` (hasta 2048 caracteres URL-safe) y `query` (hasta 160
caracteres, sin controles). Repetir parámetros, añadir parámetros desconocidos
o cambiar el contexto del cursor devuelve 400 `INVALID_PAGINATION`. El cliente
reinicia con cursor vacío. No se interpreta `query` como SQL: `%` y `_` se buscan
literalmente, mediante subcadena sin distinción de mayúsculas.

```json
{"data": [], "pagination": {"nextPageToken": ""}}
```

El cursor vacío es el fin de la colección. Cada consulta obtiene `pageSize + 1`
filas y no expone la fila adicional. Se ordena por nombre/título y después ID;
miembros usa nombre/UID e incidencias fecha de creación descendente/ID
ascendente, conservando el orden previo. Las comparaciones
SQL y ORDER BY usan la misma tupla y colación. No se usa OFFSET ni se descargan
todos los registros para cortarlos en memoria. Se puede cambiar el tamaño de
página manteniendo el cursor, pero no usuario, organización, recurso ni búsqueda.

El formato binario interno del cursor está versionado y se codifica para URL;
el cliente debe tratarlo como cadena opaca. Su huella detecta cambios de contexto,
no es una firma ni una credencial. No contiene tokens de sesión ni secretos.
Cada solicitud vuelve a consultar usuario, membresía, rol y sedes. Alterar una
posición jamás sustituye esos filtros. La clave de anclaje viaja completa: borrar
la fila de la página anterior no impide continuar. No se conserva una instantánea
entre solicitudes: altas, bajas o cambios de nombre pueden modificar el recorrido;
actualizar vuelve a la primera página. Los totales corresponden al momento de la
consulta, no a una reserva inmutable del conjunto.

Workspace acepta únicamente `pageSize`; los siguientes cursores se consumen en
las rutas de cada lista. Las incidencias consultan directamente las sedes activas
permitidas; ya no se filtran por una página previa de sedes. WORKER sigue viendo
solo sus reportes. GET de miembros devuelve 403 a CHEF/WORKER, nunca una lista
vacía que parezca autorizada.

Compatibilidad: las rutas existentes sin query conservan hasta 100 resultados y
la forma de `data`; se añaden metadatos. Al usar parámetros el tamaño por defecto
es 25; los nuevos endpoints usan 25 incluso sin query. Publicar primero backend
y después frontend. No cambia configuración, esquema, migraciones ni escrituras;
esta implementación no se despliega ni modifica la base de producción por sí sola.
Los índices del esquema existente se conservan; antes de crecer fuera del piloto
se deben medir planes reales para decidir índices adicionales de orden/búsqueda.

La primitiva técnica `shared/application/pagination` tiene dos consumidores reales,
organizaciones y checklists. Las consultas y permisos siguen dentro de cada feature.
El handler interpreta HTTP, aplicación autoriza y coordina, y PostgreSQL filtra y
limita. No se añadió un repositorio genérico.

Verificación: 137 organizaciones, sedes, miembros y cursos; nombres repetidos,
búsqueda fuera de los primeros 100, límites y parámetros inválidos, anclaje borrado,
roles, tenant ajeno, revocación entre páginas e incidencias en la sede 137.
Las pruebas locales usan PostgreSQL 14 temporal por socket privado y esquemas
`bpm_test`; CI mantiene PostgreSQL 18. Nunca usan Railway ni cuentas reales.

Referencias: [orden y límites PostgreSQL](https://www.postgresql.org/docs/18/queries-limit.html)
y [AIP-158](https://google.aip.dev/158) para cursores, autorización por solicitud y
compatibilidad. El contrato BPM define su propia validación estricta 1–100, en
lugar de adoptar la coerción de tamaños recomendada por AIP.
