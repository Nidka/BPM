# Checklists: primer recorrido operativo

## Alcance implementado

`internal/features/checklists` contiene dominio, casos de uso, PostgreSQL y HTTP.
No añade tablas ni altera `V1__baseline.sql`. Se conecta en el contenedor de
aplicación, usando el pool y el verificador Firebase ya existentes.

1. `ADMIN` del tenant o `SUPER_ADMIN` crea una plantilla con UUID de cliente,
   preguntas y versión 1; se publica en la misma transacción. Una versión
   publicada no se edita.
2. Un miembro activo descarga hasta 100 plantillas publicadas del tenant para
   caché offline.
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
- El catálogo y los ítems tienen topes para el piloto. Una expansión requerirá
  paginación explícita, sin cambiar el aislamiento por tenant.

## Pruebas

Las pruebas unitarias cubren roles, validación, versión optimista y cálculo de
puntaje. Las pruebas HTTP cubren sesión, errores de forma y sanitización. La
prueba PostgreSQL utiliza `TEST_DATABASE_URL` solo si apunta a una base llamada
`bpm_test`; instala V1 en un esquema temporal y verifica aislamiento, reintentos,
auditoría e inmutabilidad del envío. Nunca se ejecuta sobre producción.
