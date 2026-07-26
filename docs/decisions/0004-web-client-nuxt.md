# ADR-0004: Segundo cliente web con Nuxt 3 + Tailwind, desacoplado del backend

**Estado:** Aceptada, no iniciada (2026-07-26)

## Contexto

El roadmap original solo contemplaba un cliente Android nativo. Surgió la
necesidad de dos features para las que un cliente web es más apropiado que
agregar más pantallas a la app móvil de entrada rápida durante la partida:
importar decks desde Moxfield (flujo de "pegar una URL y listo", más cómodo
en desktop) y ver estadísticas post-partida (visualización de datos, también
más cómoda en pantallas grandes).

## Decisión

- Framework: **Nuxt 3 + Tailwind CSS**.
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
- **Orden de trabajo**: primero el backend real de lo que este cliente va a
  mostrar, después el scaffolding de Nuxt. Se evaluó scaffoldear en paralelo
  con datos mockeados, pero se descartó para no duplicar trabajo (mock →
  reemplazar por real).

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

## Consecuencias

- El cliente Nuxt necesita su propio proceso desplegado (a diferencia de un
  SPA estático) — hay que decidir dónde/cómo correrlo en producción (Docker
  propio, similar al de `backend/`, es la opción más consistente con el resto
  del repo, pero no está decidido todavía).
- Nada de esto está creado aún: ni `web/`, ni el scaffolding de Nuxt. El
  trabajo previo (backend de decks + import de Moxfield) ya está resuelto;
  falta el motor de partida + estadísticas reales antes de que la pantalla de
  estadísticas de este cliente tenga datos reales que mostrar.

## Referencias

- `docs/roadmap/TASKS.md`, sección "Stage 4b: Cliente Web (Nuxt)"
- [ADR-0003](0003-cors-permisivo-en-dev.md) (CORS, prerrequisito técnico)
