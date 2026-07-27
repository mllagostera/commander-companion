# Commander Companion

Aplicación para partidas de Magic: The Gathering — formato Commander. Un
backend en Go es el dueño de todo el estado real (auth, decks, partidas,
estadísticas) y lo consumen dos clientes independientes que no comparten
código entre sí: un **cliente Android nativo** (Kotlin/Compose), pensado
para trackear vida *durante* la partida con la app en la mesa —
la prioridad ahí es que cualquier acción tome menos de dos segundos — y un
**cliente web** (Nuxt), pensado para lo que se hace mejor en desktop:
importar decks de Moxfield y revisar estadísticas post-partida.

Este documento es el punto de entrada para cualquier persona o IA que empiece a trabajar en el repo. Léelo antes de tocar código.

---

## 1. Filosofía del proyecto

La prioridad NO es tener cientos de funcionalidades. La prioridad es que cualquier acción durante una partida pueda hacerse en menos de dos segundos. Todo gira en torno a tres pilares: **simplicidad, velocidad, datos**. Ver detalle en [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md).

## 2. Estructura del repo

```
commander-companion/
├── backend/              # API en Go (Fiber + PostgreSQL + sqlc + goose)
│   ├── cmd/api/          # entrypoint (main.go)
│   ├── internal/         # módulos: auth, users, decks, games, game-actions,
│   │                     #   playgroups, statistics, sync, websocket, common
│   ├── migrations/       # migraciones goose
│   ├── configs/          # configuración por entorno (aún vacío)
│   ├── sqlc.yaml         # config de generación de repositorios
│   └── Makefile          # build, run, test, lint, migrate, generate-sql
├── android/              # Cliente Android nativo (Kotlin, Compose, Hilt, Room, Retrofit)
│   └── app/src/main/java/com/commandercompanion/
│       ├── data/
│       │   ├── remote/     # CommanderApi.kt/AuthApi.kt (Retrofit), DTOs, interceptores
│       │   ├── repository/ # GameRepository, DeckRepository — deciden Room vs. backend
│       │   ├── local/      # Room (DAOs, entities)
│       │   └── session/    # SessionManager (DataStore)
│       ├── presentation/ # screens, viewmodels, navegación, tema — van directo contra repository/API
│       └── core/         # DI (Hilt) y utilidades
│       # nota: no hay capa `domain/` todavía (casos de uso), ver docs/roadmap/TASKS.md Stage 4
├── web/                  # Cliente web (Nuxt 4 SSR + Tailwind), ver ADR-0004
│   ├── server/           # capa Nitro (BFF): único lugar que toca cookies de sesión
│   │   └── api/          # auth/{register,login,google,logout,session}, backend/[...path] (proxy autenticado)
│   └── app/              # srcDir de Nuxt 4
│       ├── pages/        # login, register, index (dashboard), decks (import Moxfield), statistics
│       ├── composables/  # useAuth, useDecks, useStatistics, useApi, useGoogleIdentity
│       └── middleware/   # auth.global.ts (guard de rutas)
├── docs/                 # ver sección 8 para el índice completo, documento por documento
│   ├── roadmap/          # ROADMAP.md (visión/etapas) y TASKS.md (checklist de progreso, fuente de verdad del estado real)
│   ├── architecture/     # ARCHITECTURE.md (principios y patrones)
│   ├── database/         # schema.dbml (fuente de verdad del modelo de datos)
│   ├── api/               # openapi.yaml (fuente de verdad del contrato REST)
│   ├── decisions/        # ADRs 0001-0010 (decisiones técnicas y su porqué)
│   ├── diagrams/         # diagramas Mermaid adicionales (ER, máquina de estados, navegación Android)
│   ├── ux/                # casos-de-uso.md, wireframes.md
│   └── frontend/          # notas específicas de cliente (vacío por ahora)
├── tools/
│   └── auth-test/        # página HTML standalone para probar el flujo de auth a mano (no es parte del producto)
└── docker-compose.yml    # db + api + web, para probar el stack completo local
```

## 3. Las 4 fuentes de verdad

Antes de asumir cómo funciona algo, consulta el documento correspondiente — no el código de otro módulo por analogía, y no memoria de conversaciones previas:

| Fuente | Ubicación | Qué define |
|---|---|---|
| DBML | `docs/database/schema.dbml` | Esquema de BD, tipos, relaciones |
| OpenAPI 3.1 | `docs/api/openapi.yaml` | Contrato único backend ↔ Android **y** Web |
| Mermaid | `docs/architecture/`, `docs/diagrams/` | Arquitectura, flujos, estados |
| ADR | `docs/decisions/` | Decisiones técnicas y su porqué |

Regla: si vas a cambiar cómo se comunican backend y Android, edita primero `openapi.yaml`. Si cambias el modelo de datos, edita primero `schema.dbml` y luego crea la migración goose.

## 4. Cómo proceder (para agentes de IA)

1. **Lee `docs/roadmap/TASKS.md` primero.** Es la lista de tareas pendientes organizada por etapa, con el estado real auditado contra el código (no contra lo que "debería" existir).
2. **No confíes en que algo está terminado solo porque el archivo existe.** Gran parte del backend es scaffolding: los `service.go` de varios módulos devuelven datos dummy en vez de usar el repositorio inyectado. Verifica leyendo el código antes de asumir que una función hace lo que su nombre sugiere.
3. **Sigue el patrón de capas ya establecido:**
   - Backend: `Handler` (transporte HTTP/WS) → `Service` (lógica de negocio, sin dependencias de infraestructura) → `Repository` (sqlc, acceso a datos). Ver [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md).
   - Android: MVVM + UDF — `presentation/` (Compose + ViewModel) → `data/repository/` (decide Room vs. backend, ver `GameRepository`) → `data/remote|local/` (Retrofit/Room). La capa `domain/` (casos de uso) de Clean Architecture todavía no existe — los `ViewModel` van directo contra el repositorio (o, en auth, directo contra `AuthApi`); ver ADR-0009 y `docs/roadmap/TASKS.md` Stage 4.
   - Web: SSR con una capa Nitro (BFF) en el medio — el navegador nunca ve tokens ni llama a la API Go directo. Ver [web/README.md](web/README.md) y ADR-0004.
4. **Respeta el orden de trabajo sugerido** al final de `TASKS.md`, salvo indicación explícita del usuario de priorizar otra cosa.
5. **Si tomas una decisión técnica no trivial** (elegir una librería, un patrón, una estructura de datos que no estaba ya definida), regístrala como ADR en `docs/decisions/`.
6. **Si el trabajo toca el contrato API o el esquema de BD**, actualiza `openapi.yaml` / `schema.dbml` (y la migración correspondiente) como parte del mismo cambio, no después.

## 5. Cómo actualizar `docs/roadmap/TASKS.md`

`TASKS.md` es un documento vivo — se actualiza en el mismo cambio que resuelve la tarea, no en un paso aparte.

- Marca `- [x]` **solo** cuando la tarea esté funcionalmente completa (compila y funciona), no cuando el archivo simplemente exista o compile con un stub.
- Si una tarea queda a medias, déjala en `- [ ]` y añade una nota entre paréntesis explicando qué falta exactamente (ver ejemplos ya presentes en el documento, p. ej. "GameViewModel.kt — el archivo existe pero está vacío").
- Si durante el trabajo descubres una tarea nueva que no estaba listada (una dependencia, un caso borde, deuda técnica), añádela en la sección de la etapa correspondiente en vez de dejarla suelta en la conversación.
- **No borres tareas completadas** — son el historial de progreso del proyecto. Si una tarea deja de tener sentido (se descarta el enfoque), táchala explicando por qué en vez de eliminarla.
- Actualiza la línea `**Última revisión:**` al final de la sesión de trabajo con la fecha del día.
- Si una tarea completada cambia una de las 4 fuentes de verdad, confirma que ese archivo (`schema.dbml`, `openapi.yaml`, diagrama, ADR) quedó actualizado antes de marcarla como hecha.

## 6. Quality gates (GitHub Actions)

En `.github/workflows/` hay cuatro pipelines que corren en cada push/PR (cada uno con un job `changes`/`dorny-paths-filter` que siempre reporta un check, así ninguno queda "colgado" en PRs que no tocan su carpeta):

- **`backend-ci.yml`**: gofmt + `go vet`, `golangci-lint`, verifica que `sqlc generate` no deje diffs sin commitear, build + `go test -race` + aplica las migraciones goose contra un Postgres real del job, y `hadolint` sobre `backend/Dockerfile`.
- **`android-ci.yml`**: Android Lint, tests unitarios (`testDebugUnitTest`), `assembleDebug`.
- **`web-ci.yml`**: ESLint + typecheck (`vue-tsc`) + `nuxt build` (SSR), y `hadolint` sobre `web/Dockerfile`.
- **`docs-ci.yml`**: valida que las fuentes de verdad sigan siendo válidas — Spectral lint sobre `openapi.yaml` y `schema.dbml` compilando a SQL.

**Nota sobre branch protection**: los checks *requeridos* en `main` hoy son solo 8, todos de `backend-ci.yml`/`android-ci.yml`/`docs-ci.yml` — `web-ci.yml` se agregó después de configurar branch protection y sus checks no están en la lista de requeridos todavía. Detalle no obvio: el job `hadolint (Dockerfile)` de `web-ci.yml` tiene el mismo nombre que el de `backend-ci.yml`, así que hoy cualquiera de los dos satisface ese check requerido (GitHub matchea por nombre de job, no por workflow) — pero `eslint, typecheck y nuxt build` de `web-ci.yml` no es requerido por nada.

Antes de dar una tarea por terminada, estos gates deben pasar en local (o al menos no introducir issues nuevos) para lo que toques: `make lint` / `make test` en backend, `./gradlew lintDebug testDebugUnitTest` en Android. Requieren que el repo esté conectado a GitHub para ejecutarse; localmente son solo los comandos del Makefile / Gradle.

## 7. Comandos útiles

**Todo el stack** (desde la raíz, requiere Docker):
```
docker compose up --build   # db + api + web en contenedores (ver web/README.md)
```
La primera vez hay que aplicar las migraciones del backend (no corren solas
dentro del contenedor), ver `backend/Makefile` (`make migrate-up`).

**Backend** (`cd backend`):
```
make run                 # levantar la API localmente
make test                # go test -race ./...
make lint                # golangci-lint
make generate-sql        # regenerar repos con sqlc tras editar query.sql
make migrate-up          # aplicar migraciones goose
```

**Web** (`cd web`):
```
npm install
npm run dev              # http://localhost:3000, requiere la API corriendo aparte
```

**Android** (`cd android`):
```
./gradlew assembleDebug  # build
./gradlew test           # tests unitarios
./gradlew connectedAndroidTest  # tests instrumentados
```

## 8. Hub de documentación

Este es el índice completo — todo documento del repo debería estar
enlazado desde acá. Si agregás un documento nuevo bajo `docs/` (o un README
de un módulo nuevo), agregalo también a esta lista en el mismo cambio.

**Empezar por acá:**

- [docs/roadmap/TASKS.md](docs/roadmap/TASKS.md) — **la fuente de verdad del estado real**, auditada contra el código, no contra lo que "debería" existir. Léelo antes que cualquier otra cosa.
- [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) — visión, filosofía, etapas de alto nivel (documento de intención original; para el estado real ver TASKS.md).
- [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) — las 4 fuentes de verdad, principios de diseño y patrones de capas (backend, Android, Web).

**Fuentes de verdad (ver tabla en la sección 3):**

- [docs/database/schema.dbml](docs/database/schema.dbml) — esquema de la base de datos.
- [docs/api/openapi.yaml](docs/api/openapi.yaml) — contrato REST único backend ↔ Android/Web.

**Diagramas (`docs/diagrams/`):**

- [docs/diagrams/er-diagram.md](docs/diagrams/er-diagram.md) — diagrama entidad-relación completo, generado a partir del DBML.
- [docs/diagrams/game-state-machine.md](docs/diagrams/game-state-machine.md) — máquina de estados de una partida (`games`) y del ciclo de vida de cada jugador (`game-actions`).
- [docs/diagrams/android-navigation-flow.md](docs/diagrams/android-navigation-flow.md) — grafo de navegación real del cliente Android (`NavHost`/rutas).

**Casos de uso y wireframes (`docs/ux/`):**

- [docs/ux/casos-de-uso.md](docs/ux/casos-de-uso.md) — las 5 operaciones centrales del producto, columna "Hoy" (código real) vs. "Objetivo".
- [docs/ux/wireframes.md](docs/ux/wireframes.md) — wireframes ASCII de las 6 pantallas reales del cliente Android.

**ADRs — decisiones técnicas (`docs/decisions/`):**

- [0001 — Estrategia de autenticación (JWT + refresh token)](docs/decisions/0001-auth-jwt-refresh-token-strategy.md)
- [0002 — Google Sign-In como proveedor adicional](docs/decisions/0002-google-sign-in.md)
- [0003 — CORS permisivo en dev](docs/decisions/0003-cors-permisivo-en-dev.md)
- [0004 — Cliente web con Nuxt 4 + Tailwind](docs/decisions/0004-web-client-nuxt.md)
- [0005 — Protocolo de sincronización en vivo por WebSocket](docs/decisions/0005-websocket-protocol.md)
- [0006 — Backend en Go con Fiber](docs/decisions/0006-go-fiber-backend.md)
- [0007 — PostgreSQL como base de datos principal](docs/decisions/0007-postgresql.md)
- [0008 — sqlc + goose (acceso a datos y migraciones)](docs/decisions/0008-sqlc-goose.md)
- [0009 — Android nativo vs. cross-platform](docs/decisions/0009-android-nativo-vs-crossplatform.md)
- [0010 — Monolito modular vs. microservicios](docs/decisions/0010-monolito-modular-vs-microservicios.md)

**READMEs por módulo:**

- [backend/README.md](backend/README.md) — setup, comandos (`make`), stack del backend.
- [web/README.md](web/README.md) — setup, sesión vía Nitro/BFF, estructura del cliente Nuxt.
- [tools/auth-test/README.md](tools/auth-test/README.md) — herramienta HTML standalone para probar el flujo de auth a mano (no es parte del producto).

No hay un README propio para `android/` todavía — su documentación vive
repartida entre [docs/ux/wireframes.md](docs/ux/wireframes.md),
[docs/diagrams/android-navigation-flow.md](docs/diagrams/android-navigation-flow.md),
[ADR-0009](docs/decisions/0009-android-nativo-vs-crossplatform.md) y
`docs/roadmap/TASKS.md` (Stage 4/5).
