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
│       ├── domain/       # use cases + repository interfaces that data/ implements
│       ├── presentation/ # screens, viewmodels, navigation, theme — depend on domain/
│       └── core/         # DI (Hilt) and utilities
│       # note: the auth screens (Login/Register/Settings) deliberately stay outside domain/, see docs/roadmap/TASKS.md Stage 4
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
│   ├── decisions/        # ADRs (technical decisions and their rationale)
│   ├── diagrams/         # additional Mermaid diagrams (ER, state machine, Android navigation)
│   └── ux/                # use-cases.md, wireframes.md, screenshots.md
└── docker-compose.yml    # db + api + web, to try the full stack locally
```

## 3. The 4 sources of truth

Before assuming how something works, consult the corresponding document — not another module's code by analogy, and not memory from previous conversations. Full detail (what each one defines, where it lives) in [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) §"The 4 Sources of Truth": DBML, OpenAPI 3.1, Mermaid, ADR.

Rule: if you're going to change how backend and Android communicate, edit `openapi.yaml` first. If you change the data model, edit `schema.dbml` first and then create the goose migration.

## 4. How to proceed (for AI agents)

> Full instructions for agents live in [AGENTS.md](AGENTS.md) — CodeGraph usage, language rules, architecture rules per area, quality gates and git workflow. What follows is the summary.

1. **Read `docs/roadmap/TASKS.md` first.** It's the list of pending tasks organized by stage, with real status audited against the code (not against what "should" exist). It's a compact checklist on purpose — for the narrative behind any item (why, gotchas, how it was verified), see [docs/roadmap/DECISIONS-LOG.md](docs/roadmap/DECISIONS-LOG.md); don't read the whole log up front, only the entries you actually need.
2. **Don't trust that something is finished just because the file exists.** Much of the backend is scaffolding: several modules' `service.go` return dummy data instead of using the injected repository. Verify by reading the code before assuming a function does what its name suggests.
3. **Follow the already-established layer pattern** — backend (Handler → Service → Repository), Android (MVVM + UDF, presentation → domain → data), Web (SSR + Nitro BFF). Full detail in [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) §"System Architecture".
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

In `.github/workflows/` there are five quality pipelines that run on every push/PR (the four per-area ones each carry a `changes`/`dorny-paths-filter` job that always reports a check, so none is left "hanging" on PRs that do not touch its folder; `architecture-ci.yml` deliberately has none, see below). Every check name is in English; they double as the identifiers branch protection matches on, so renaming one means updating the protection settings in the same pass.

- **`backend-ci.yml`**: gofmt + `go vet`, `golangci-lint`, verifies that `sqlc generate` leaves no uncommitted diffs, build + `go test -race` + applies the goose migrations against a real Postgres from the job, and `hadolint` on `backend/Dockerfile`.
- **`android-ci.yml`**: Android Lint, unit tests (`testDebugUnitTest`), `assembleDebug`, and `string resources translated in every locale`.
- **`web-ci.yml`**: ESLint + typecheck (`vue-tsc`) + `nuxt build` (SSR), `i18n keys resolve in every locale`, and `hadolint` on `web/Dockerfile`.
- **`docs-ci.yml`**: validates that the sources of truth remain valid — Spectral lint on `openapi.yaml`, `schema.dbml` compiling to SQL, and `schema.dbml matches the migrations`, which builds the schema twice in one Postgres (once with goose, once from the compiled DBML) and diffs `information_schema`. Compiling only proves the DBML is well-formed; this proves it is true. It compares tables, columns, types and nullability — not indexes or constraints, since DBML cannot express partial indexes and `dbml2sql` renders unique constraints as unique indexes, so those comparisons report differences that aren't drift. It runs when `backend/migrations/**` changes too, not just `docs/`: a migration alone can invalidate the DBML, which is how `deck_resync_jobs` went undocumented for weeks.
- **`architecture-ci.yml`**: runs [`.github/scripts/check-architecture.sh`](.github/scripts/check-architecture.sh), which enforces what `ARCHITECTURE.md` and `PROJECT-STRUCTURE.md` otherwise only assert in prose — the Handler layer free of persistence, no slice reaching into another slice's sqlc `Queries`, SQL confined to `query.sql`, the web client never bypassing the Nitro BFF, every registered route present in `openapi.yaml` and no phantom paths, and every document under `docs/` linked from this hub (§8). It also warns, without blocking, when `TASKS.md`'s review date has fallen behind the code. Unlike the other four it has **no path filter**: the invariants are cross-cutting (a backend edit breaks the OpenAPI check, a docs edit breaks the hub check) and the whole script is a handful of greps. It exists because of a 2026-09-03 audit: every rule with a check was at 100% compliance, while three that lived only in prose — link new ADRs from the hub, keep `TASKS.md` fresh, keep `ARCHITECTURE.md` true — had all drifted despite being clearly written. In a repo worked on only by agents, each starting cold, a rule that isn't checked isn't a rule.


**Architecture guardrails.** Beyond the pipelines above, the layering itself is enforced by four mechanisms, each owning the rules it can express natively and **nothing checked twice**: `depguard` in [`backend/.golangci.yml`](backend/.golangci.yml) (a handler must not import a database driver), [Konsist](android/app/src/test/java/com/commandercompanion/architecture/ArchitectureTest.kt) as unit tests in `testDebugUnitTest` (Android layering, the enumerated auth exception, and two ratchets that freeze known debt so it can only shrink), `eslint-plugin-boundaries` plus `no-restricted-imports` in [`web/eslint.config.mjs`](web/eslint.config.mjs) (`app/` ↔ `server/`, both directions), and [`check-architecture.sh`](.github/scripts/check-architecture.sh) for everything that is not an import. The three native tools ride gates that already exist, so they cost no new CI job. Full breakdown of who owns what, and why the cross-slice `Queries` rule can only live in the script, in [AGENTS.md](AGENTS.md) §7.

**The i18n checks** (`.github/scripts/check-i18n-{web,android}.mjs`) exist because a missing translation key fails nothing else: on the web Vue renders the key itself and the page still returns 200; on Android the string falls back to the default locale and shows up in Spanish. Both run in seconds without a build. Each verifies that the locales define the same keys and that every key referenced in the source resolves. Android resources deliberately left untranslated must carry `tools:ignore="MissingTranslation"`, the same annotation Android Lint honours.

**Note on branch protection**: the *required* checks on `main` are 8, all from `backend-ci.yml`/`android-ci.yml`/`docs-ci.yml` — `web-ci.yml` was added after branch protection was configured and its checks aren't in the required list yet, and neither are the two new i18n ones. Non-obvious detail: the `hadolint (Dockerfile)` job in `web-ci.yml` has the same name as the one in `backend-ci.yml`, so today either one satisfies that required check (GitHub matches by job name, not by workflow) — but `eslint, typecheck and nuxt build` from `web-ci.yml` isn't required by anything.

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

- [AGENTS.md](AGENTS.md) — how any AI agent works in this repo: CodeGraph, language rules, standards, quality gates, git workflow. Read it before touching code.
- [docs/roadmap/TASKS.md](docs/roadmap/TASKS.md) — **the source of truth for real status**, audited against the code, not against what "should" exist. Read it before anything else. Kept deliberately compact — one line per item.
- [docs/roadmap/DECISIONS-LOG.md](docs/roadmap/DECISIONS-LOG.md) — the narrative behind TASKS.md's items (why, gotchas, verification, dates), plus the chronological audit/session history. Read only the entries you need, not the whole file.
- [docs/roadmap/ROADMAP.md](docs/roadmap/ROADMAP.md) — vision, philosophy, high-level stages (original intent document; for real status see TASKS.md).
- [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) — the 4 sources of truth, design principles, and layer patterns (backend, Android, Web).
- [docs/architecture/PROJECT-STRUCTURE.md](docs/architecture/PROJECT-STRUCTURE.md) — the file-by-file map of the three apps and the step-by-step for adding an endpoint, a migration, a page or a screen.

**Sources of truth (see §3, full detail in ARCHITECTURE.md):**

- [docs/database/schema.dbml](docs/database/schema.dbml) — database schema.
- [docs/api/openapi.yaml](docs/api/openapi.yaml) — single REST contract backend ↔ Android/Web.

**Diagrams (`docs/diagrams/`):**

- [docs/diagrams/er-diagram.md](docs/diagrams/er-diagram.md) — full entity-relationship diagram, generated from the DBML.
- [docs/diagrams/game-state-machine.md](docs/diagrams/game-state-machine.md) — state machine of a game (`games`) and of each player's lifecycle (`game-actions`).
- [docs/diagrams/android-navigation-flow.md](docs/diagrams/android-navigation-flow.md) — the Android client's actual navigation graph (`NavHost`/routes).

**Use cases and wireframes (`docs/ux/`):**

- [docs/ux/use-cases.md](docs/ux/use-cases.md) — the game-loop's 5 core operations ("Today" column, actual code, vs. "Target") plus decks/playgroups/tournaments.
- [docs/ux/wireframes.md](docs/ux/wireframes.md) — ASCII wireframes of the Android client's 6 actual screens.
- [docs/ux/screenshots.md](docs/ux/screenshots.md) — real screenshots of every web client flow (auth, dashboard, decks, playgroups, tournaments, statistics, local life tracker, settings).

**ADRs — technical decisions (`docs/decisions/`):**

- [docs/decisions/TEMPLATE.md](docs/decisions/TEMPLATE.md) — copy this to start a new ADR; it carries the "link it from this hub" reminder at the point of use.
- [0001 — Authentication strategy (JWT + refresh token)](docs/decisions/0001-auth-jwt-refresh-token-strategy.md)
- [0002 — Google Sign-In as an additional provider](docs/decisions/0002-google-sign-in.md)
- [0003 — Permissive CORS in dev](docs/decisions/0003-permissive-cors-in-dev.md)
- [0004 — Web client with Nuxt 4 + Tailwind](docs/decisions/0004-web-client-nuxt.md)
- [0005 — Live sync protocol over WebSocket](docs/decisions/0005-websocket-protocol.md)
- [0006 — Backend in Go with Fiber](docs/decisions/0006-go-fiber-backend.md)
- [0007 — PostgreSQL as the primary database](docs/decisions/0007-postgresql.md)
- [0008 — sqlc + goose (data access and migrations)](docs/decisions/0008-sqlc-goose.md)
- [0009 — Native Android vs. cross-platform](docs/decisions/0009-android-native-vs-crossplatform.md)
- [0010 — Modular monolith vs. microservices](docs/decisions/0010-modular-monolith-vs-microservices.md)
- [0011 — Migration naming strategy and statistics recompute](docs/decisions/0011-migration-strategy-and-statistics-recalculation.md)
- [0012 — Email verification at registration with Resend](docs/decisions/0012-email-verification-resend.md)
- [0013 — Proxy-join and action authorization](docs/decisions/0013-proxy-join-and-action-authorization.md)
- [0014 — Web internationalization with @nuxtjs/i18n](docs/decisions/0014-web-internationalization.md)
- [0015 — Deployment infrastructure (Render + Vercel + Supabase)](docs/decisions/0015-deployment-infrastructure.md)
- [0016 — Standalone Swiss-format tournaments](docs/decisions/0016-swiss-tournament-format.md)
- [0017 — Friend requests and profile QR](docs/decisions/0017-friends-system.md)
- [0018 — Admin role and user moderation](docs/decisions/0018-admin-role-and-user-moderation.md)
- [0019 — The Android domain layer owns its types](docs/decisions/0019-android-domain-owns-its-types.md)
- [0020 — `GET /health` reports which build is answering](docs/decisions/0020-build-provenance-in-health.md)

**READMEs per module:**

- [backend/README.md](backend/README.md) — setup, commands (`make`), backend stack.
- [android/README.md](android/README.md) — setup, required JDK, Google Sign-In, Android client structure.
- [web/README.md](web/README.md) — setup, session via Nitro/BFF, Nuxt client structure.
