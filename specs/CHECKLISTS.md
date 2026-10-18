# Checklists: primer recorrido operativo

## Alcance implementado

`internal/features/checklists` contiene dominio, casos de uso, PostgreSQL y HTTP.
No añade tablas ni altera `V1__baseline.sql`. Se conecta en el contenedor de
aplicación, usando el pool y el verificador Firebase ya existentes.

1. `ADMIN` del tenant o `SUPER_ADMIN` crea una plantilla con UUID de cliente,
   preguntas y versión 1; se publica en la misma transacción. Una versión
   publicada no se edita.
2. Un miembro activo descarga páginas de plantillas publicadas completas del tenant.
   Los cursores permiten continuar sin limitar el catálogo a las primeras 100.
3. Un miembro con acceso a una sede activa inicia una ejecución con ID,
   `clientOperationId` y `clientCreatedAt` generados por el cliente. Repetir
   exactamente el alta devuelve la ejecución existente sin duplicar auditoría.
4. El ejecutor guarda el conjunto completo de respuestas usando
   `rowVersion` e `Idempotency-Key`. Un reintento idéntico devuelve la respuesta
   original; la misma clave con otros datos o una versión obsoleta da 409.
5. El ejecutor envía el borrador. El servidor valida las preguntas requeridas,
   calcula el cumplimiento y congela la ejecución. `NOT_APPLICABLE` y `TEXT`
   no entran al denominador. Debe haber al menos una pregunta evaluable.

Todas las escrituras relevantes generan `audit_events` en la misma transacción.
Las respuestas HTTP usan `{ "data": ... }`; los errores usan `errors[]` con
`code`, `message` y `fields` cuando corresponda, más `requestId`.

## Rutas

Prefijo `/api/v1/organizations/{organizationID}`. Todas requieren Firebase ID
token con correo verificado y usuario BPM activo.

| Método | Ruta | Permiso |
|---|---|---|
| GET | `/checklists` | miembro activo |
| GET | `/checklists/{templateID}` | miembro activo; última versión publicada completa |
| POST | `/checklists` | ADMIN o SUPER_ADMIN |
| POST | `/checklist-runs` | miembro asignado a sede, o ADMIN |
| GET | `/checklist-runs/{runID}` | ejecutor o supervisor con acceso a sede |
| PUT | `/checklist-runs/{runID}/answers` | ejecutor; `Idempotency-Key` |
| POST | `/checklist-runs/{runID}/submit` | ejecutor; `Idempotency-Key` |

El contrato de cuerpos y respuestas vive en `internal/core/httpserver/docs/openapi.yaml`.
El cliente debe mantener una cola local con el mismo ID, clave y cuerpo entre
reintentos; después de 409 `VERSION_CONFLICT`, debe recuperar el run y resolver
el conflicto antes de enviar. Las claves de idempotencia quedan registradas
durante al menos 24 horas; no deben reciclarse.

## Límites conscientes de esta primera base

- Aún no hay edición de plantilla ni publicación de versión 2, asignaciones
  programadas ni listado paginado de ejecuciones. No hay endpoint de archivo.
- La carga de evidencias privadas en R2 será otro módulo. Hasta entonces la API
  no permite crear ítems con evidencia obligatoria. Si existiera uno en datos
  previos, se rechaza el envío de una respuesta fallida para no omitir el control.
- El guardado de respuestas representa el borrador completo, no un parche. Las
  evidencias que se incorporen después exigirán preservar su vínculo al editar.
- El catálogo está paginado; cada plantilla conserva hasta 100 preguntas según
  el contrato de publicación. La ejecución y sus respuestas no se paginan.

## Pruebas

Las pruebas unitarias cubren roles, validación, versión optimista y cálculo de
puntaje. Las pruebas HTTP cubren sesión, errores de forma y sanitización. La
prueba PostgreSQL utiliza `TEST_DATABASE_URL` solo si apunta a una base llamada
`bpm_test`; instala V1 en un esquema temporal y verifica aislamiento, reintentos,
auditoría e inmutabilidad del envío. Nunca se ejecuta sobre producción.

## Catálogo paginado y detalle

GET `/checklists` usa el mismo contrato `pageSize`, `pageToken`, `query` y
`pagination.nextPageToken` de [ORGANIZATIONS.md](ORGANIZATIONS.md). La búsqueda
incluye nombre, área y descripción. La consulta selecciona primero las plantillas
y su versión publicada más reciente en un CTE con límite de página + 1. Después
incorpora todas las preguntas de esas versiones, sin LIMIT sobre las filas unidas.
No se corta una plantilla ni se omiten sus preguntas al alcanzar 10 000 filas.

GET `/checklists/{templateID}` exige autorización nueva del tenant y devuelve
la última versión publicada con todas sus preguntas; 404 si no existe dentro del
contexto autorizado. No admite query. Permite enlaces directos aunque la plantilla
no esté en la página cargada del catálogo. No inicia ni modifica ejecuciones.

Se preservan las rutas existentes y sus `data`, con hasta 100 plantillas sin query
por compatibilidad. No se cambian tablas ni variables. Las pruebas recorren 137
plantillas con 100 preguntas cada una, comprueban el detalle posterior a la página
inicial, tenant ajeno y membresía revocada entre páginas. PostgreSQL temporal y CI
se describen en ORGANIZATIONS.md.
