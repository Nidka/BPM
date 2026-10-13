# Componentes compartidos

## Objetivo

Evitar duplicación sin convertir `shared` o `utils` en dependencias globales sin
límites. Compartir código no significa que todas las features deban conocerlo.

## Ubicación

```text
internal/shared/
  domain/       conceptos de dominio realmente comunes y estables
  application/ contratos y primitivas de casos de uso transversales
  utils/        funciones puras, pequeñas y sin conocimiento del negocio
```

Las carpetas se crean al aparecer el primer componente válido. El README de
`internal/shared` reserva la frontera, pero no se agregan APIs especulativas.

## Regla de promoción

Un código pasa a `shared` cuando cumple todo lo siguiente:

1. Tiene al menos dos consumidores reales.
2. Representa exactamente el mismo concepto y comportamiento.
3. Puede nombrarse sin usar palabras vagas como `common`, `misc` o `helper`.
4. No obliga a una feature a importar detalles de otra.
5. Tiene pruebas propias y una API menor que las duplicaciones que reemplaza.

Antes de cumplir estas condiciones permanece privado dentro de la feature.

## Qué puede vivir en `utils`

Solo funciones deterministas y sin estado, basadas en la biblioteca estándar;
por ejemplo, normalizaciones técnicas o validaciones de formato que no expresen
una regla del negocio. `utils` no puede importar `core`, `features`, pgx, Chi ni
SDK externos.

No pertenecen a `utils`:

- autorización, tenancy o roles;
- transacciones y repositorios;
- DTO HTTP o respuestas de error;
- configuración y variables de entorno;
- clientes Firebase, R2, correo o PostgreSQL;
- reglas sanitarias o cálculos de cumplimiento.

Estos elementos viven en su feature o en el paquete específico de `core`.

## Dependencias permitidas

```text
features/domain      -> shared/domain
features/application -> shared/domain + shared/application
features/adapters    -> su feature + core técnico necesario
core                 -> shared cuando consume una primitiva transversal
shared               -> biblioteca estándar; nunca features
```

No se crean interfaces para cada struct. Las interfaces se declaran en el lado
consumidor y solo cuando existe una frontera sustituible: persistencia,
proveedores externos, reloj, generador de identificadores o transacción.
