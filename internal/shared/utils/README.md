# Utils

Esta carpeta queda reservada para utilidades puras y reutilizadas por varias
features. No se agregan archivos Go hasta que exista un consumidor real.

Una utilidad aquí:

- no mantiene estado;
- no conoce el dominio BPM;
- no importa `core`, `features` ni SDK externos;
- tiene un nombre específico y pruebas unitarias;
- reemplaza duplicación comprobada.

La política completa está en `specs/SHARED_COMPONENTS.md`.
