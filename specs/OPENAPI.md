# OpenAPI y Swagger UI

## Rutas

- `/docs/`: interfaz Swagger UI embebida en el binario.
- `/openapi.yaml`: contrato OpenAPI 3.0.3 versionado.

La documentación es pública y no contiene secretos. Los endpoints protegidos
siguen exigiendo un ID token válido de Firebase. Swagger UI permite ingresarlo
con **Authorize** usando el esquema `FirebaseBearer`.

## Ubicación y mantenimiento

El contrato y su adaptador viven en `internal/core/httpserver/docs` porque son
infraestructura HTTP compartida. Cada feature debe actualizar `openapi.yaml` en
el mismo cambio que agregue o modifique una ruta, DTO, estado HTTP o código de
error público.

La UI usa recursos embebidos; no depende de CDN ni carga JavaScript de terceros.
Solo las rutas bajo `/docs` reciben una política CSP específica para ejecutar
esos recursos. El resto de la API conserva `default-src 'none'`.

## Verificación

```bash
curl -i http://localhost:8080/openapi.yaml
open http://localhost:8080/docs/
```

En Railway:

```text
https://bpm-service-production.up.railway.app/docs/
https://bpm-service-production.up.railway.app/openapi.yaml
```
