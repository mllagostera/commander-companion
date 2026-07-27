# Arquitectura y Principios de Diseño

Este documento describe la arquitectura y los principios fundamentales de diseño del proyecto **Commander Companion**.

## Las 4 Fuentes de Verdad

Para garantizar que el proyecto pueda escalar y que el desarrollo paralelo (incluyendo la colaboración con IAs) sea eficiente y coherente, el sistema se basa en cuatro pilares documentales:

1. **DBML (Database Markup Language):**
   Define el esquema de la base de datos, tipos, relaciones, índices y restricciones.
   Ubicación: `docs/database/schema.dbml`

2. **OpenAPI 3.1:**
   Es el contrato único e inviolable entre el backend (Go) y los clientes (Android y Web/Nuxt, ver [ADR-0004](../decisions/0004-web-client-nuxt.md)). Cualquier cambio en la comunicación debe reflejarse primero aquí.
   Ubicación: `docs/api/openapi.yaml`

3. **Mermaid (Diagramas):**
   Utilizados para documentar arquitectura, flujos de datos, máquinas de estado y comportamiento de los sistemas.
   Ubicación: Dentro de los Markdowns en `docs/architecture/` y `docs/diagrams/`.

4. **ADR (Architecture Decision Records):**
   Documentación estructurada sobre decisiones técnicas clave, contexto, opciones consideradas y consecuencias.
   Ubicación: `docs/decisions/`

---

## Arquitectura del Sistema

### Backend (Go)
- **Patrón:** Monolito Modular.
- **Estructura Interna:** Clean Architecture enfocada en casos de uso (Service).
  - `Handler`: Capa de transporte (HTTP/REST, Websocket).
  - `Service`: Lógica de negocio (pura, sin dependencias de infraestructura).
  - `Repository`: Persistencia y acceso a datos.

### Cliente (Android)
- **Patrón:** Clean Architecture + MVVM + UDF (Unidirectional Data Flow).
- **Módulos Lógicos:**
  - `Presentation`: UI con Jetpack Compose y ViewModels.
  - `Domain`: casos de uso e interfaces de repositorios — capa todavía no
    materializada; los `ViewModel` van directo contra `data/repository/` o,
    en auth, directo contra la API (ver `docs/roadmap/TASKS.md`, Stage 4).
  - `Data`: repositorios (`GameRepository`, `DeckRepository`) que deciden
    qué persiste en Room (local) y qué llama al backend real (Retrofit,
    `CommanderApi`) — no es una capa puramente de paso, ya tiene la lógica
    de "qué va a cada lado".

### Cliente Web (Nuxt)
- **Patrón:** SSR con una capa BFF (Nitro) en el medio — ver
  [ADR-0004](../decisions/0004-web-client-nuxt.md), `web/README.md`.
- **Módulos Lógicos:**
  - `server/` (Nitro): único lugar que toca cookies de sesión (`httpOnly`)
    y hace de proxy autenticado hacia la API Go — el navegador nunca ve un
    token ni llama a la API Go directamente.
  - `app/`: código Nuxt propiamente dicho — `pages/` (rutas), `composables/`
    (`useAuth`, `useDecks`, `useStatistics`, cada uno envuelve su parte del
    contrato REST), `middleware/` (gating de rutas autenticadas).
- 100% desacoplado de Android: comparten el contrato REST
  (`docs/api/openapi.yaml`), no código ni componentes.
