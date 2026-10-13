# Shared

Espacio reservado para primitivas reutilizables entre dos o más features.

No es un contenedor de código sobrante. Cada componente compartido debe tener un
nombre concreto, pruebas y dependencias mínimas. Las reglas de promoción están
en `specs/SHARED_COMPONENTS.md`.

Subcarpetas permitidas cuando exista una necesidad real:

- `domain`: conceptos de dominio transversales y estables;
- `application`: contratos técnicos de casos de uso;
- `utils`: funciones puras sin reglas de negocio ni infraestructura.
