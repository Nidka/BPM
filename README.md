# Backend BPM

El esquema inicial de PostgreSQL se encuentra en `database/schema.sql`.
La guía para instalarlo en Railway está en `RAILWAY.md`.

Debe ejecutarse sobre una base vacía:

```bash
psql -v ON_ERROR_STOP=1 -d bpm_database -f database/schema.sql
```

El archivo es autocontenido: crea tablas, claves, índices, vistas y triggers
dentro de una sola transacción.

## Datos básicos incluidos

- Empresa: razón social, nombre comercial, identificación fiscal y contacto.
- Local: nombre, dirección, referencia, distrito, provincia, departamento,
  teléfono, correo y zona horaria.
- Usuario: correo, nombre y teléfono opcional.
- Proveedor: identificación fiscal, dirección y contacto comercial.
- Operación: categoría de producto, referencia de documento de recepción e
  instrucciones de limpieza.
- Contenido: duración estimada de cursos y descripción de instrumentos.

Los teléfonos se almacenan como texto para admitir `+51`, espacios y ceros
iniciales. No se agregaron datos personales de contacto a los participantes de
investigación para mantener su seudonimización.
# BPM
