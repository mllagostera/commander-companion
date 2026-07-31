# Commander Companion

An app for Magic: The Gathering games — Commander format. A Go backend
owns all the real state (auth, decks, games, statistics), consumed by
two independent clients that share no code with each other: a
**native Android client** (Kotlin/Compose), designed to track life
*during* the game with the app on the table —
the priority there is that any action takes less than two seconds — and a
**web client** (Nuxt), designed for what's done better on desktop:
importing decks from Moxfield and reviewing post-game statistics.

This document is the entry point for any person or AI starting to work on the repo. Read it before touching code.

---

## 1. Project philosophy

The priority is NOT to have hundreds of features. The priority is that any action during a game can be done in under two seconds. Everything revolves around three pillars: **simplicity, speed, data**. See details in [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md).

## 2. Repo structure

```
commander-companion/
├── backend/              # Go API (Fiber + PostgreSQL + sqlc + goose)
│   ├── cmd/api/          # entrypoint (main.go)
│   ├── internal/         # modules: auth, users, decks, games, game-actions,
│   │                     #   playgroups, statistics, sync, websocket, common
│   ├── migrations/       # goose migrations
│   ├── configs/          # per-environment configuration (still empty)
│   ├── sqlc.yaml         # repository generation config
│   └── Makefile          # build, run, test, lint, migrate, generate-sql
├── android/              # Native Android client (Kotlin, Compose, Hilt, Room, Retrofit)
│   └── app/src/main/java/com/commandercompanion/
│       ├── data/
│       │   ├── remote/     # CommanderApi.kt/AuthApi.kt (Retrofit), DTOs, interceptors
│       │   ├── repository/ # GameRepository, DeckRepository — decide Room vs. backend
│       │   ├── local/      # Room (DAOs, entities)
│       │   └── session/    # SessionManager (DataStore)
│       ├── presentation/ # screens, viewmodels, navigation, theme — go straight against repository/API
│       └── core/         # DI (Hilt) and utilities
│       # note: there is no `domain/` layer yet (use cases), see docs/roadmap/TASKS.md Stage 4
├── web/                  # Web client (Nuxt 4 SSR + Tailwind), see ADR-0004
│   ├── server/           # Nitro layer (BFF): the only place that touches session cookies
│   │   └── api/          # auth/{register,login,google,logout,session}, backend/[...path] (authenticated proxy)
│   └── app/              # Nuxt 4 srcDir
│       ├── pages/        # login, register, index (dashboard), decks (import Moxfield), statistics
│       ├── composables/  # useAuth, useDecks, useStatistics, useApi, useGoogleIdentity
│       └── middleware/   # auth.global.ts (route guard)
├── docs/                 # see section 8 for the full index, document by document
│   ├── roadmap/          # ROADMAP.md (vision/stages) and TASKS.md (progress checklist, source of truth for real status)
│   ├── architecture/     # ARCHITECTURE.md (principles and patterns)
│   ├── database/         # schema.dbml (source of truth for the data model)
│   ├── api/               # openapi.yaml (source of truth for the REST contract)
│   ├── decisions/        # ADRs 0001-0010 (technical decisions and their rationale)
│   ├── diagrams/         # additional Mermaid diagrams (ER, state machine, Android navigation)
│   ├── ux/                # casos-de-uso.md, wireframes.md
│   └── frontend/          # client-specific notes (empty for now)
├── tools/
│   └── auth-test/        # standalone HTML page to test the auth flow by hand (not part of the product)
└── docker-compose.yml    # db + api + web, to try the full stack locally
```

## 3. The 4 sources of truth

Before assuming how something works, consult the corresponding document — not another module's code by analogy, and not memory from previous conversations:

| Source | Location | What it defines |
|---|---|---|
| DBML | `docs/database/schema.dbml` | DB schema, types, relations |
| OpenAPI 3.1 | `docs/api/openapi.yaml` | Single contract backend ↔ Android **and** Web |
| Mermaid | `docs/architecture/`, `docs/diagrams/` | Architecture, flows, states |
| ADR | `docs/decisions/` | Technical decisions and their rationale |

Rule: if you're going to change how backend and Android communicate, edit `openapi.yaml` first. If you change the data model, edit `schema.dbml` first and then create the goose migration.

## 4. How to proceed (for AI agents)

1. **Read `docs/roadmap/TASKS.md` first.** It's the list of pending tasks organized by stage, with real status audited against the code (not against what "should" exist).
2. **Don't trust that something is finished just because the file exists.** Much of the backend is scaffolding: several modules' `service.go` return dummy data instead of using the injected repository. Verify by reading the code before assuming a function does what its name suggests.
3. **Follow the already-established layer pattern:**
   - Backend: `Handler` (HTTP/WS transport) → `Service` (business logic, no infrastructure dependencies) → `Repository` (sqlc, data access). See [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md).
   - Android: MVVM + UDF — `presentation/` (Compose + ViewModel) → `data/repository/` (decides Room vs. backend, see `GameRepository`) → `data/remote|local/` (Retrofit/Room). Clean Architecture's `domain/` layer (use cases) doesn't exist yet — `ViewModel`s go straight against the repository (or, in auth, straight against `AuthApi`); see ADR-0009 and `docs/roadmap/TASKS.md` Stage 4.
   - Web: SSR with a Nitro layer (BFF) in between — the browser never sees tokens or calls the Go API directly. See [web/README.md](web/README.md) and ADR-0004.
4. **Respect the suggested work order** at the end of `TASKS.md`, unless the user explicitly asks to prioritize something else.
5. **If you make a non-trivial technical decision** (choosing a library, a pattern, a data structure that wasn't already defined), record it as an ADR in `docs/decisions/`.
6. **If the work touches the API contract or the DB schema**, update `openapi.yaml` / `schema.dbml` (and the corresponding migration) as part of the same change, not afterward.

## 5. How to update `docs/roadmap/TASKS.md`

`TASKS.md` is a living document — it gets updated in the same change that resolves the task, not as a separate step.

- Mark `- [x]` **only** when the task is functionally complete (it builds and works), not when the file simply exists or compiles with a stub.
- If a task is left half-done, keep it as `- [ ]` and add a note in parentheses explaining exactly what's missing (see examples already present in the document, e.g. "GameViewModel.kt — the file exists but is empty").
- If during the work you discover a new task that wasn't listed (a dependency, an edge case, technical debt), add it to the corresponding stage section instead of leaving it loose in the conversation.
- **Don't delete completed tasks** — they are the project's progress history. If a task stops making sense (the approach is dropped), strike it through explaining why instead of deleting it.
- Update the `**Last reviewed:**` line at the end of the work session with the current date.
- If a completed task changes one of the 4 sources of truth, confirm that file (`schema.dbml`, `openapi.yaml`, diagram, ADR) was updated before marking it done.

## 6. Quality gates (GitHub Actions)

In `.github/workflows/` there are four pipelines that run on every push/PR (each with a `changes`/`dorny-paths-filter` job that always reports a check, so none is left "hanging" on PRs that don't touch its folder):

- **`backend-ci.yml`**: gofmt + `go vet`, `golangci-lint`, verifies that `sqlc generate` leaves no uncommitted diffs, build + `go test -race` + applies the goose migrations against a real Postgres from the job, and `hadolint` on `backend/Dockerfile`.
- **`android-ci.yml`**: Android Lint, unit tests (`testDebugUnitTest`), `assembleDebug`.
- **`web-ci.yml`**: ESLint + typecheck (`vue-tsc`) + `nuxt build` (SSR), and `hadolint` on `web/Dockerfile`.
- **`docs-ci.yml`**: validates that the sources of truth remain valid — Spectral lint on `openapi.yaml` and `schema.dbml` compiling to SQL.

**Note on branch protection**: the *required* checks on `main` today are only 8, all from `backend-ci.yml`/`android-ci.yml`/`docs-ci.yml` — `web-ci.yml` was added after branch protection was configured and its checks aren't in the required list yet. Non-obvious detail: the `hadolint (Dockerfile)` job in `web-ci.yml` has the same name as the one in `backend-ci.yml`, so today either one satisfies that required check (GitHub matches by job name, not by workflow) — but `eslint, typecheck and nuxt build` from `web-ci.yml` isn't required by anything.

Before considering a task done, these gates must pass locally (or at least not introduce new issues) for whatever you touched: `make lint` / `make test` in backend, `./gradlew lintDebug testDebugUnitTest` in Android. They require the repo to be connected to GitHub to run; locally they're just the Makefile / Gradle commands.

## 7. Useful commands

**Full stack** (from the root, requires Docker):
```
docker compose up --build   # db + api + web in containers (see web/README.md)
```
The first time, the backend migrations need to be applied by hand (they
don't run automatically inside the container), see `backend/Makefile` (`make migrate-up`).

**Backend** (`cd backend`):
```
make run                 # run the API locally
make test                # go test -race ./...
make lint                # golangci-lint
make generate-sql        # regenerate repos with sqlc after editing query.sql
make migrate-up          # apply goose migrations
```

**Web** (`cd web`):
```
npm install
npm run dev              # http://localhost:3000, requires the API running separately
```

**Android** (`cd android`):
```
./gradlew assembleDebug  # build
./gradlew test           # unit tests
./gradlew connectedAndroidTest  # instrumented tests
```

## 8. Documentation hub

This is the full index — every document in the repo should be
linked from here. If you add a new document under `docs/` (or a README
for a new module), add it to this list too in the same change.

**Start here:**

- [docs/roadmap/TASKS.md](docs/roadmap/TASKS.md) — **the source of truth for real status**, audited against the code, not against what "should" exist. Read it before anything else.
- [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) — vision, philosophy, high-level stages (original intent document; for real status see TASKS.md).
- [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) — the 4 sources of truth, design principles, and layer patterns (backend, Android, Web).

**Sources of truth (see table in section 3):**

- [docs/database/schema.dbml](docs/database/schema.dbml) — database schema.
- [docs/api/openapi.yaml](docs/api/openapi.yaml) — single REST contract backend ↔ Android/Web.

**Diagrams (`docs/diagrams/`):**

- [docs/diagrams/er-diagram.md](docs/diagrams/er-diagram.md) — full entity-relationship diagram, generated from the DBML.
- [docs/diagrams/game-state-machine.md](docs/diagrams/game-state-machine.md) — state machine of a game (`games`) and of each player's lifecycle (`game-actions`).
- [docs/diagrams/android-navigation-flow.md](docs/diagrams/android-navigation-flow.md) — the Android client's actual navigation graph (`NavHost`/routes).

**Use cases and wireframes (`docs/ux/`):**

- [docs/ux/casos-de-uso.md](docs/ux/casos-de-uso.md) — the product's 5 core operations, "Today" column (actual code) vs. "Target".
- [docs/ux/wireframes.md](docs/ux/wireframes.md) — ASCII wireframes of the Android client's 6 actual screens.

**ADRs — technical decisions (`docs/decisions/`):**

- [0001 — Authentication strategy (JWT + refresh token)](docs/decisions/0001-auth-jwt-refresh-token-strategy.md)
- [0002 — Google Sign-In as an additional provider](docs/decisions/0002-google-sign-in.md)
- [0003 — Permissive CORS in dev](docs/decisions/0003-cors-permisivo-en-dev.md)
- [0004 — Web client with Nuxt 4 + Tailwind](docs/decisions/0004-web-client-nuxt.md)
- [0005 — Live sync protocol over WebSocket](docs/decisions/0005-websocket-protocol.md)
- [0006 — Backend in Go with Fiber](docs/decisions/0006-go-fiber-backend.md)
- [0007 — PostgreSQL as the primary database](docs/decisions/0007-postgresql.md)
- [0008 — sqlc + goose (data access and migrations)](docs/decisions/0008-sqlc-goose.md)
- [0009 — Native Android vs. cross-platform](docs/decisions/0009-android-nativo-vs-crossplatform.md)
- [0010 — Modular monolith vs. microservices](docs/decisions/0010-monolito-modular-vs-microservicios.md)

**READMEs per module:**

- [backend/README.md](backend/README.md) — setup, commands (`make`), backend stack.
- [android/README.md](android/README.md) — setup, required JDK, Google Sign-In, Android client structure.
- [web/README.md](web/README.md) — setup, session via Nitro/BFF, Nuxt client structure.
- [tools/auth-test/README.md](tools/auth-test/README.md) — standalone HTML tool to test the auth flow by hand (not part of the product).
