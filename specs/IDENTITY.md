# Feature de identidad y tenancy

## Alcance implementado

El primer recorrido vertical es `GET /api/v1/me`. Resuelve la sesión del usuario
sin almacenar ni recibir contraseñas en la API.

```text
ID token de Firebase
  → middleware Authenticate
  → UID verificado en el contexto
  → GetCurrentProfile
  → application_users
  → membresías, organizaciones y locales activos
  → respuesta segura de sesión
```

Firebase demuestra la identidad. PostgreSQL es la fuente de verdad para el
estado interno, rol de plataforma, organizaciones, roles empresariales y locales
permitidos. El cliente nunca decide su rol ni su tenant efectivo.

Correo/contraseña y Google usan el mismo endpoint. El frontend intercambia el
login Google por un **ID token Firebase**; el backend no acepta contraseñas,
access tokens Google ni tokens Google sin ese intercambio. El verificador Admin
comprueba el token y su revocación, independientemente del proveedor. El UID
es la clave de acceso; un correo coincidente no habilita otro UID ni fusiona cuentas.
No se crean usuarios o membresías automáticamente al iniciar sesión con Google.

Para habilitar el endpoint se configuran `FIREBASE_ENABLED=true`,
`FIREBASE_PROJECT_ID` y `FIREBASE_SERVICE_ACCOUNT_JSON_BASE64`. Con Firebase
deshabilitado la ruta existe, pero devuelve `503 AUTHENTICATION_UNAVAILABLE`;
esto permite desarrollar la base técnica sin simular una identidad insegura.

El Project ID del despliegue es `app-bpm-af59c`. La API key pública de la app web
no es la cuenta de servicio: generar su JSON en Firebase → Configuración →
Cuentas de servicio, mantenerlo fuera de Git y configurar su base64 únicamente
como secreto sellado del backend. Habilitar Firebase después de instalar ese
secreto y dar de alta el UID exacto de prueba en `application_users`.

## Solicitud

```http
GET /api/v1/me
Authorization: Bearer <firebase-id-token>
```

No recibe `organizationId`: devuelve todas las membresías activas asociadas al
UID verificado. La selección de organización activa se implementará como el
siguiente caso de uso y deberá comprobar que pertenezca a esta lista.

## Respuesta 200

```json
{
  "data": {
    "id": "firebase-uid",
    "email": "usuario@example.com",
    "displayName": "Usuario BPM",
    "platformRole": "USER",
    "organizations": [
      {
        "id": "organization-id",
        "legalName": "Restaurante BPM S.A.C.",
        "tradeName": "Restaurante BPM",
        "role": "ADMIN",
        "locations": [
          {
            "id": "location-id",
            "name": "Local Centro",
            "address": "Av. Principal 123",
            "district": "Trujillo",
            "province": "Trujillo",
            "department": "La Libertad",
            "timezone": "America/Lima"
          }
        ]
      }
    ]
  }
}
```

Los arreglos vacíos se devuelven como `[]`, no como `null`. Las membresías,
organizaciones y locales inactivos no se exponen.

## Errores

- `401 UNAUTHENTICATED`: falta el token o Firebase lo rechaza.
- `403 USER_NOT_REGISTERED`: el UID no existe en `application_users`.
- `403 USER_INACTIVE`: el usuario existe pero está deshabilitado.
- `500 INTERNAL_ERROR`: fallo interno sanitizado.
- `503 AUTHENTICATION_UNAVAILABLE`: Firebase está deshabilitado o no fue
  configurado en el proceso.

El formato completo está definido en `API_ERRORS.md`.

## Organización del código

```text
internal/features/identity/
  domain/                 perfil, roles y errores de negocio
  application/            caso de uso y puerto del repositorio
  adapters/postgres/      consulta de usuario, membresías y locales
  adapters/http/          DTO, traducción de errores y ruta protegida
  module.go               composición interna del feature
```

## Próximos recorridos del feature

1. Selección y resolución del tenant activo.
2. Listado de organizaciones y locales autorizado.
3. Invitación y administración de miembros.
4. Registro de dispositivos FCM.
5. Autorización reutilizable por rol, tenant y local.

