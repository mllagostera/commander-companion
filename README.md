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
│   ├── roadmap/          # ROADMAP.md (vision/stages), TASKS.md (compact status checklist) and DECISIONS-LOG.md (narrative history)
│   ├── architecture/     # ARCHITECTURE.md (principles and patterns)
│   ├── database/         # schema.dbml (source of truth for the data model)
│   ├── api/               # openapi.yaml (source of truth for the REST contract)
│   ├── decisions/        # ADRs 0001-0010 (technical decisions and their rationale)
│   ├── diagrams/         # additional Mermaid diagrams (ER, state machine, Android navigation)
│   └── ux/                # casos-de-uso.md, wireframes.md
└── docker-compose.yml    # db + api + web, to try the full stack locally
```

## 3. The 4 sources of truth

Before assuming how something works, consult the corresponding document — not another module's code by analogy, and not memory from previous conversations. Full detail (what each one defines, where it lives) in [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) §"The 4 Sources of Truth": DBML, OpenAPI 3.1, Mermaid, ADR.

Rule: if you're going to change how backend and Android communicate, edit `openapi.yaml` first. If you change the data model, edit `schema.dbml` first and then create the goose migration.

## 4. How to proceed (for AI agents)

1. **Read `docs/roadmap/TASKS.md` first.** It's the list of pending tasks organized by stage, with real status audited against the code (not against what "should" exist). It's a compact checklist on purpose — for the narrative behind any item (why, gotchas, how it was verified), see [docs/roadmap/DECISIONS-LOG.md](docs/roadmap/DECISIONS-LOG.md); don't read the whole log up front, only the entries you actually need.
2. **Don't trust that something is finished just because the file exists.** Much of the backend is scaffolding: several modules' `service.go` return dummy data instead of using the injected repository. Verify by reading the code before assuming a function does what its name suggests.
3. **Follow the already-established layer pattern** — backend (Handler → Service → Repository), Android (MVVM + UDF, no `domain/` layer yet), Web (SSR + Nitro BFF). Full detail in [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) §"System Architecture".
4. **Respect the suggested work order** at the end of `TASKS.md`, unless the user explicitly asks to prioritize something else.
5. **If you make a non-trivial technical decision** (choosing a library, a pattern, a data structure that wasn't already defined), record it as an ADR in `docs/decisions/`.
6. **If the work touches the API contract or the DB schema**, update `openapi.yaml` / `schema.dbml` (and the corresponding migration) as part of the same change, not afterward.

## 5. How to update `docs/roadmap/TASKS.md`

`TASKS.md` is a living document — it gets updated in the same change that resolves the task, not as a separate step. Since 2026-08-01 it's deliberately kept **compact**: one short line per item (status + what + a file/module pointer). Narrative — why, gotchas hit, how it was verified, dates, exact user requests — goes in [docs/roadmap/DECISIONS-LOG.md](docs/roadmap/DECISIONS-LOG.md) instead, not back into TASKS.md; that split exists specifically to keep a new session from having to load the whole project history just to check status.

- Mark `- [x]` **only** when the task is functionally complete (it builds and works), not when the file simply exists or compiles with a stub.
- If a task is left half-done, keep it as `- [ ]` and add a short note in parentheses explaining what's missing (see examples already present in the document).
- If during the work you discover a new task that wasn't listed (a dependency, an edge case, technical debt), add it to the corresponding stage section instead of leaving it loose in the conversation.
- **Don't delete completed tasks** — they are the project's progress history. If a task stops making sense (the approach is dropped), strike it through explaining why instead of deleting it.
- Keep each line short. If an item needs more than a sentence or two of context, add a dated entry to `DECISIONS-LOG.md` under the matching stage and link to it from the TASKS.md line instead of inlining the detail.
- Update the `**Last reviewed:**` line at the end of the work session with the current date, and add the corresponding entry to `DECISIONS-LOG.md`'s audit/session history.
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

- [docs/roadmap/TASKS.md](docs/roadmap/TASKS.md) — **the source of truth for real status**, audited against the code, not against what "should" exist. Read it before anything else. Kept deliberately compact — one line per item.
- [docs/roadmap/DECISIONS-LOG.md](docs/roadmap/DECISIONS-LOG.md) — the narrative behind TASKS.md's items (why, gotchas, verification, dates), plus the chronological audit/session history. Read only the entries you need, not the whole file.
- [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) — vision, philosophy, high-level stages (original intent document; for real status see TASKS.md).
- [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) — the 4 sources of truth, design principles, and layer patterns (backend, Android, Web).

**Sources of truth (see §3, full detail in ARCHITECTURE.md):**

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
