# ADR-0004: Segundo cliente web con Nuxt 4 + Tailwind, desacoplado del backend

**Estado:** Aceptada, esqueleto iniciado (2026-07-26, actualizada 2026-07-27)

## Contexto

El roadmap original solo contemplaba un cliente Android nativo. Surgió la
necesidad de dos features para las que un cliente web es más apropiado que
agregar más pantallas a la app móvil de entrada rápida durante la partida:
importar decks desde Moxfield (flujo de "pegar una URL y listo", más cómodo
en desktop) y ver estadísticas post-partida (visualización de datos, también
más cómoda en pantallas grandes).

## Decisión

- Framework: **Nuxt 4 + Tailwind CSS**. Se optó por la última versión mayor
  disponible en vez de fijarse en Nuxt 3 — criterio general del proyecto:
  arrancar cada dependencia nueva en su versión más actualizada posible en
  vez de una anterior "más probada", salvo que haya una razón concreta para
  no hacerlo. Nuxt 4 cambia principalmente la convención de estructura de
  carpetas (todo el código de la app vive bajo `app/`) respecto a Nuxt 3; no
  afecta ninguna de las demás decisiones de este ADR.
- **100% desacoplado del backend**: solo consume la API REST vía HTTP (la
  misma API que usa Android), sin lógica compartida ni acoplamiento de
  despliegue. Esto ya estaba habilitado por [ADR-0003](0003-cors-permisivo-en-dev.md)
  (CORS) — sin eso, un frontend en otro origen no podría llamar a la API
  desde el navegador.
- Modo de renderizado: **SSR** (Nuxt completo, con Nitro corriendo como
  proceso Node en runtime) — no SPA estática. A diferencia de
  `tools/auth-test/` (HTML estático de un archivo, sin servidor propio), este
  cliente sí necesita su propio proceso de servidor desplegado.
- Ubicación en el repo: `web/`, al mismo nivel que `android/` y `backend/`.
- Gestor de paquetes: **npm** (ya es la herramienta que usan los workflows de
  CI existentes para Spectral y `@dbml/cli` — cero herramienta nueva que
  instalar/aprender en el pipeline).
- **Orden de trabajo original**: primero el backend real de lo que este
  cliente va a mostrar, después el scaffolding de Nuxt. Se evaluó
  scaffoldear en paralelo con datos mockeados, pero se descartó para no
  duplicar trabajo (mock → reemplazar por real). **Adelantado parcialmente
  el 2026-07-27** a pedido explícito del usuario: se scaffoldeó ya el
  esqueleto de auth (login email/password + Google), porque ese flujo no
  depende del motor de partida/estadísticas pendiente en el backend. El
  resto (import de Moxfield, estadísticas) sigue esperando ese trabajo de
  backend antes de construirse.

## Alternativas consideradas

- **Nuxt en modo SPA (`ssr: false`)**: se compila a estáticos, sin proceso
  Node en runtime, desplegable en cualquier CDN — más liviano y más acorde a
  "superliviano". Fue la recomendación inicial, pero el usuario prefirió
  **SSR completo** explícitamente al decidir.
- **Extender el cliente Android** en vez de agregar un cliente web: se
  descartó porque el caso de uso (importar un deck pegando una URL, revisar
  gráficos de estadísticas) encaja mejor en desktop/navegador que en la app
  de partida, que está optimizada para velocidad durante el juego, no para
  tareas de gestión/consulta.
- **pnpm** en vez de npm: más rápido/liviano, pero se priorizó no introducir
  una herramienta nueva en el toolchain de CI ya existente.
- **Nuxt 3** (versión originalmente decidida el 2026-07-26): se cambió a
  Nuxt 4 al día siguiente porque el criterio del proyecto es arrancar con la
  versión mayor más actualizada de cada librería nueva, no la anterior más
  probada — no hubo ningún problema técnico con Nuxt 3, fue puramente
  alinear la decisión con ese criterio antes de que hubiera más código
  construido encima.

## Consecuencias

- El cliente Nuxt necesita su propio proceso desplegado (a diferencia de un
  SPA estático) — hay que decidir dónde/cómo correrlo en producción (Docker
  propio, similar al de `backend/`, es la opción más consistente con el resto
  del repo, pero no está decidido todavía).
- Esqueleto inicial ya creado en `web/` (2026-07-27): Nuxt 4 + Tailwind +
  login (email/password + Google) + una pantalla protegida mínima. Sigue
  pendiente lo que dependía del backend de partidas/estadísticas: import de
  Moxfield y la pantalla de estadísticas, que esperan el motor de partida +
  estadísticas reales antes de tener datos que mostrar.

## Referencias

- `docs/roadmap/TASKS.md`, sección "Stage 4b: Cliente Web (Nuxt)"
- [ADR-0003](0003-cors-permisivo-en-dev.md) (CORS, prerrequisito técnico)
