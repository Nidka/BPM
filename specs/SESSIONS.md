# Política de sesiones BPM

## Decisión y referencias

El requisito de BPM establece sesiones de 8 horas. Se aplica un máximo absoluto
de 8 horas desde la autenticación Firebase y 15 minutos de inactividad. Ambos
límites se comprueban en el servidor. La renovación del ID token no reinicia
ninguno de los dos por sí sola.

Referencias: [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#session-expiration),
[Firebase: sesiones y revocación](https://firebase.google.com/docs/auth/admin/manage-sessions),
[Firebase: verificación de tokens y auth_time](https://firebase.google.com/docs/auth/admin/verify-id-tokens).
Los plazos son una política del producto ajustada al piloto, no una obligación
universal de Firebase. Deben revisarse según el riesgo y el uso de equipos compartidos.

## Implementación

1. Firebase Admin verifica firma, proyecto, vencimiento y revocación del ID token.
2. El adaptador extrae `auth_time` verificado como `AuthenticatedAt`. No se usa
   el `iat` del token renovado ni un reloj/fecha enviados por el cliente.
3. `identity/application/Sessions` comprueba el plazo absoluto y consulta su puerto.
4. PostgreSQL registra UID, `auth_time`, última actividad y revocación. No guarda
   contraseñas, ID tokens ni refresh tokens. La migración es V2; V1 permanece intacta.
5. Un upsert condicional comprueba y actualiza la actividad atómicamente. Dos
   réplicas comparten la misma política; no hay caché de sesiones por instancia.
6. Un contexto vencido conserva su fila. Renovar Firebase no vuelve a crearlo.
   Se exige autenticación nueva. Un error de almacenamiento responde 503 y no
   permite llegar al módulo; el vencimiento responde 401 `SESSION_EXPIRED`.

El middleware se comparte en identidad, organizaciones y checklists. Los controles
de usuario BPM activo, correo verificado, empresa, rol y sedes continúan en sus
casos de uso; una sesión técnica válida no concede permisos empresariales.

La clave del contexto es `(UID, auth_time)`, no un identificador de dispositivo.
Dos autenticaciones del mismo UID en el mismo segundo comparten este contexto;
también lo comparten clientes que reutilicen la misma autenticación Firebase.
No se ofrece gestión independiente de dispositivos. Tras logout, si una nueva
autenticación coincide en ese segundo, debe repetirse en un segundo posterior.

## Actividad y rutas

- `GET /api/v1/me` comprueba acceso sin prolongar la sesión: sirve para sondeos
  periódicos y restauración móvil.
- Las demás solicitudes protegidas cuentan como actividad. El cliente no puede
  enviar una fecha arbitraria para prolongar el plazo.
- `POST /api/v1/auth/activity` valida y registra actividad; no recibe cuerpo ni
  permisos. El frontend lo llama por interacción real, como máximo una vez por
  minuto. No se envía por temporizadores ni por renovación de Firebase.
- `POST /api/v1/auth/logout` revoca el contexto Firebase utilizado. No revoca
  automáticamente las otras autenticaciones de la cuenta.
- Respuestas autorizadas incluyen `X-BPM-Session-Expires-At` y
  `X-BPM-Session-Idle-Expires-At`, en segundos Unix. CORS las expone y el proxy
  permite únicamente valores numéricos acotados.

Un atacante que ya tenga una credencial válida podría generar actividad; por
eso el límite absoluto, la revocación y la autorización en cada petición son
necesarios además del límite de inactividad.

## Cliente KMP

El frontend observa toques, teclado y desplazamiento. Los diálogos tienen su
propio observador y los campos notifican cambios del teclado virtual. El aviso
aparece durante el último minuto y el acceso local se retira al vencer el plazo.
El backend sigue siendo la autoridad incluso si se manipula el reloj del cliente.

La restauración de un refresh token móvil siempre confirma `/me`. Un 401 elimina
las credenciales locales y no reintenta escrituras. Logout limpia localmente
antes de esperar la revocación remota. Si no hay red, informa que el cierre remoto
no pudo confirmarse: la credencial ya emitida permanece sujeta a los límites del
servidor. El cierre local por sí solo no demuestra revocación remota.

El aviso no implementa persistencia de borradores. Los formularios actuales son
temporales y se limpian al salir de sesión. Los borradores operativos y la cola
offline deben implementarse con su contrato de persistencia, separados por usuario
y empresa; no deben enviarse mientras el servidor exija autenticación nueva.

## Configuración

```env
AUTH_SESSION_ABSOLUTE_TIMEOUT=8h
AUTH_SESSION_IDLE_TIMEOUT=15m
```

Se rechazan plazos no positivos, un plazo absoluto mayor de 8 horas o inactividad
mayor que el plazo absoluto. Omitir estas variables usa los valores anteriores.

## Despliegue y compatibilidad

Esta entrega está en ramas de trabajo. V2 se aplicó en PostgreSQL de Railway el
18/10/2026 a las 02:20:15, hora de Lima, con autorización del usuario. No se han
publicado los binarios nuevos ni cambiado variables de producción. La credencial
PostgreSQL permanece sin cambios.

### Ejecución comprobada en Railway

- Destino: base `bpm`, esquema `public`, conexión SSL.
- Respaldo previo del esquema, sin copiar registros de usuarios, en
  `tmp/operaciones/migracion-v2-20261018T071935Z/antes-esquema.sql`. La carpeta
  está ignorada por Git y los archivos tienen acceso restringido al propietario.
- Se ejecutó únicamente V2 en una transacción, con bloqueo asesor y tiempos de
  espera acotados. No se volvió a ejecutar V1 ni se creó un registro de
  migraciones ajeno al runner previsto.
- Verificados: cuatro columnas, clave primaria compuesta y dos índices válidos
  contando el de la clave primaria. La tabla nueva quedó vacía.
- Las definiciones de columnas y restricciones de las 52 tablas anteriores
  coinciden con las registradas antes de la operación.
- `/health` y `/ready` continuaron con HTTP 200; `/api/v1/me` sin token respondió
  401. La versión publicada siguió siendo `7b3f94f`.
- SHA-256 de V2 aplicada:
  `4b32b2499aa23ce115e5c5e7b96673e450cedb496a840f3c2d87b3e01b9022d0`.

V2 ya está aplicada en esta base: no volver a ejecutarla ni editar su contenido.
La creación de la tabla no activa los límites de sesión; falta publicar primero
el frontend compatible y después el backend nuevo.

Orden de publicación (el paso 2 ya se completó en esta base de Railway):

1. Verificar la fuente/commit actual en Railway y realizar el respaldo operativo
   correspondiente. La API observada el 18/10/2026 publica `7b3f94f`, que existe
   en `Nidka/BPM-service` y coincide con su `main` remoto al revisar.
2. Aplicar **únicamente V2** una vez, mediante el acceso administrativo habitual,
   con `ON_ERROR_STOP=1` y el esquema esperado. Nunca ejecutar V1 en esa base existente.
   El runner automático de migraciones sigue pendiente; no añadir un predeploy
   que ejecute V1.
3. Publicar primero el frontend preparado para `SESSION_EXPIRED`, deadlines y
   nuevas rutas. Con el backend anterior no envía actividad ni logout remoto
   porque no recibe las cabeceras de política.
4. Publicar el backend validado, preservando checklists y configuración existente.
   El proceso comprueba la tabla y sus columnas antes de abrir HTTP. Sin V2 no
   arranca; verificar `Wait for CI` y healthcheck en Railway antes de publicar.
5. Comprobar `/health`, `/ready`, HSTS, login de prueba, vencimiento, roles y logout.
   Probar restauración en Android/iOS. No usar cuentas/datos productivos para las
   pruebas automáticas de permisos.

La tabla es aditiva y no modifica usuarios, organizaciones ni registros existentes.
Una reversión al backend anterior pierde estos límites de sesión y requiere
evaluación de seguridad; no borrar la tabla para revertir binarios.

Retención: los contextos vencidos/revocados deben conservarse al menos hasta
superar su máximo absoluto para impedir reapertura. Puede programarse limpieza
de filas con `authenticated_at < now() - interval '24 hours'`; no se instaló un
job administrativo ni se eliminaron datos durante esta entrega.

## HSTS y administración externa

Backend: HSTS se activa en staging/production y, por defecto, al detectar el
entorno Railway; no depende de confiar en `X-Forwarded-Proto`. El valor explícito
`HTTP_HSTS_ENABLED` permite habilitarlo adicionalmente. Frontend: HSTS se activa
con `NODE_ENV=production` o entorno Railway, para el host actual, sin preload.
En HTTP local no se habilita por defecto. La ausencia pública observada no queda
resuelta hasta desplegar y comprobar las respuestas reales.

Firebase Admin no está configurado en el `.env` local revisado. La clave pública
del frontend no permite auditar IAM, restricciones ni proveedores administrativos.
Antes de cambiar la consola, inventariar clientes web/Android/iOS, probar con una
clave de prueba y conservar sus APIs requeridas. No se retiraron dominios,
modificaron cuotas ni restringieron claves de producción en esta entrega.

## Verificación

Pruebas de límites exactos, auth_time ausente/futuro, fallos de almacenamiento,
contextos vencidos, logout, dos réplicas y concurrencia. PostgreSQL de pruebas
exige `bpm_test` y usa esquemas aislados. La matriz de organizaciones añade
peticiones HTTP con roles/cabeceras falsificadas y casos CHEF/WORKER/ADMIN.
Firebase se sustituye por dobles en esas pruebas: no certifican una cuenta real.
