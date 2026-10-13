# Contrato de errores HTTP

Todas las respuestas de error usan el mismo cuerpo. `errors` siempre es un
arreglo, incluso cuando existe un solo error:

```json
{
  "errors": [
    {
      "code": "VALIDATION_ERROR",
      "message": "Uno o más campos son inválidos.",
      "fields": {
        "email": "El correo no tiene un formato válido.",
        "displayName": "El nombre es obligatorio."
      }
    }
  ],
  "requestId": "01J..."
}
```

## Reglas

- `code` es estable, está en mayúsculas y permite al cliente tomar decisiones.
- `message` es seguro, entendible y está en español; puede mostrarse al usuario.
- `fields` aparece únicamente cuando el error corresponde a campos de entrada.
  Sus claves usan los nombres JSON del formulario o solicitud.
- `requestId` permite correlacionar la respuesta con los logs sin revelar
  información interna.
- Nunca se serializan errores Go, SQL, nombres internos, credenciales, tokens,
  cuerpos, cadenas de conexión ni trazas.
- Varios problemas independientes pueden ocupar varios elementos de `errors`.
  Varios campos inválidos de un mismo formulario se agrupan en `fields`.

## Estados principales

| HTTP | Uso |
|---:|---|
| 400 | JSON o estructura de solicitud inválida |
| 401 | Token ausente, inválido, vencido o revocado |
| 403 | Identidad válida sin acceso, usuario no registrado o inactivo |
| 404 | Recurso inexistente dentro del tenant autorizado |
| 409 | Conflicto de versión, idempotencia o estado |
| 422 | Campos válidos sintácticamente pero rechazados por validación |
| 429 | Límite de solicitudes superado |
| 500 | Fallo interno sanitizado |
| 503 | Dependencia técnica o autenticación no disponible |

`core/httpserver/response` implementa este contrato. Cada adaptador HTTP traduce
los errores comparables de su dominio; la capa de aplicación y el dominio no
conocen códigos HTTP ni DTO JSON.

