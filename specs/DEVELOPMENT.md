# Desarrollo local

## Recarga automática

Air ejecuta la API durante el desarrollo y la reinicia cuando cambia un archivo
Go dentro de `cmd` o `internal`.

```bash
make dev
```

El objetivo instala Air en `.tools/bin` si todavía no existe. La versión está
fijada en el `Makefile` para ser compatible con Go 1.27 y permitir que todo el
equipo use el mismo ejecutable. Un archivo de control por versión fuerza la
actualización de la herramienta cuando cambie el pin.

La configuración vive en `.air.toml`:

- carga las variables de `.env`;
- compila `./cmd/api` en `tmp/air/bpm-api`;
- no reinicia por pruebas, specs o migraciones;
- envía una interrupción al proceso para activar el cierre controlado;
- conserva los errores de compilación visibles y no ejecuta un binario antiguo.

`.tools` y `tmp` son artefactos locales ignorados. Air no se incluye en
`go.mod`, Docker, CI ni Railway y nunca debe usarse como supervisor de
producción.

## Logs

En `development` y `test`, `slog` usa un handler de consola coloreado con hora,
nivel, mensaje y atributos legibles. El arranque confirma explícitamente:

1. configuración cargada y puerto solicitado;
2. conexión PostgreSQL verificada mediante `Ping`;
3. pool mínimo y máximo disponible;
4. puerto HTTP reservado correctamente;
5. URL local y endpoints `/health` y `/ready`.

El mensaje `HTTP server is ready` solo se registra después de abrir el listener.
Si el puerto está ocupado, la aplicación informa `listen HTTP on port ...` y no
muestra un estado listo incorrecto.

Después de conectar PostgreSQL y reservar el puerto, `StartupBanner` presenta
un panel ASCII panorámico de 114 columnas. El logotipo ocupa la sección
izquierda; estado, ambiente, versión, puerto y endpoints se agrupan a la
derecha; la autoría y el estado final comparten el pie. El servidor depende
únicamente del contrato
`contracts.StartupReporter`; la implementación visual se inyecta desde el
composition root. Así el transporte HTTP no conoce colores ni detalles de
presentación, y el banner puede cambiarse o desactivarse sin modificarlo.

El banner se imprime una sola vez y únicamente en `development` o `test`.

Los ambientes `staging` y `production` mantienen JSON estructurado sin códigos
ANSI. Ningún formato registra `DATABASE_URL`, contraseñas, tokens ni URLs
firmadas.

El puerto se configura exclusivamente con `PORT`:

```env
PORT=8080
```

En desarrollo la consola muestra `http://localhost:<PORT>`. En Railway se
registra el puerto interno, mientras el acceso público continúa mediante el
dominio HTTPS asignado al servicio.

## Comandos

| Comando | Uso |
|---|---|
| `make dev` | Instala Air si hace falta e inicia hot reload |
| `make tools` | Instala únicamente las herramientas locales |
| `make run` | Ejecuta la API directamente, sin hot reload |
| `make check` | Formatea, analiza y ejecuta pruebas |
