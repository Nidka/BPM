# Migraciones

Migraciones versionadas de PostgreSQL del backend BPM.

- `V1__baseline.sql` es la línea base para instalaciones nuevas.
- Los cambios siguientes usan `V2__descripcion.sql`, `V3__descripcion.sql`, etc.
- Un archivo aplicado no se edita; se crea la siguiente versión.
- La especificación del futuro runner está en `specs/MIGRATIONS.md`.

Ejecución manual sobre una base vacía:

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f migraciones/V1__baseline.sql
```

No ejecutar nuevamente el baseline sobre la base de Railway que ya contiene las
tablas.
