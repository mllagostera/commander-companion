# AGENTS.md — how to work in this repository

Instructions for any AI agent (Claude Code, Gemini CLI, Codex, Copilot, or a
sub-agent spawned by one of them) working on **Commander Companion**. It is the
vendor-neutral entry point: tool-specific files (`.claude/CLAUDE.md`,
`GEMINI.md`) are local and gitignored, this one is committed and is the shared
contract.

Humans should start at [README.md](README.md) — it explains the project itself.
This file explains *how work is done here*, not what the project is.

---

## 0. TL;DR — the ten rules

1. **Read [`docs/roadmap/TASKS.md`](docs/roadmap/TASKS.md) before anything else.** It is the audited status of the code.
2. **Use CodeGraph before grep/find/Read** (see §2). One `codegraph_explore` call usually replaces a dozen file reads.
3. **All documentation and code comments in English.** User-facing strings stay multilingual (see §3).
4. **Contract first**: touching the API → edit `openapi.yaml` first. Touching the data model → edit `schema.dbml` first, then the goose migration.
5. **Don't trust a file's existence.** Parts of the codebase were scaffolding; read the code before assuming a function does what its name says.
6. **Non-trivial technical decision → write an ADR** in `docs/decisions/`, copying [`docs/decisions/TEMPLATE.md`](docs/decisions/TEMPLATE.md).
7. **Update `TASKS.md` in the same change** that resolves the task; the narrative goes to `DECISIONS-LOG.md`.
8. **Never work directly on `main`.** Branch with a `feat/`, `fix/`, `chore/`, `ci/`, `docs/` prefix and open a PR.
9. **Before calling anything done, run `sh .github/scripts/check-architecture.sh`** — it is the checklist (§7), and CI runs the same script.
10. **`- [x]` only when it functionally works** — not when it compiles.

---

## 1. The project in 30 seconds

A Magic: The Gathering (Commander format) companion app. A Go backend owns all
real state; two independent clients consume it and **share no code with each
other**, only the REST contract.

| Area | Stack | Purpose |
|---|---|---|
| `backend/` | Go 1.25, Fiber, PostgreSQL, sqlc, goose | Owns auth, decks, games, statistics |
| `android/` | Kotlin 2.4, Compose, Hilt, Room, Retrofit | At-the-table life tracking — **any action in under 2 seconds** |
| `web/` | Nuxt 4 SSR + Nitro BFF + Tailwind | Desktop work: Moxfield imports, statistics, admin |
| `docs/` | Markdown, DBML, OpenAPI, Mermaid | The sources of truth (§4) |

The guiding priority is **not** feature count. It is *simplicity, speed, data* —
see [`docs/roadmap/ROADMAP.md`](docs/roadmap/ROADMAP.md). A feature that makes an
in-game action slower is a regression, however useful it looks.

Repo layout in detail: [README.md §2](README.md). File-by-file map, plus the
step-by-step for adding an endpoint, a migration, a page or a screen:
[`docs/architecture/PROJECT-STRUCTURE.md`](docs/architecture/PROJECT-STRUCTURE.md).

---

## 2. CodeGraph — use it before reading files

The repo is indexed by CodeGraph (there is a `.codegraph/` directory at the
root). It is a SQLite knowledge graph of every symbol, edge and file: reads are
sub-millisecond and the index trails writes by ~1s via a file watcher.

**Reach for it BEFORE grep, find, or opening files** — both when answering
questions and when about to edit code, because it returns the verbatim source
*plus* the blast radius (who calls this, what breaks) in one shot.

### Two interfaces, same output

- **MCP tool** (`.mcp.json` / `.gemini/settings.json` register it as a stdio server):

  ```
  codegraph_explore("How does refresh token rotation work?")
  codegraph_explore("GameRepository FinishGame ResolveGameOutcomeUseCase")
  ```

  If the tool is listed but deferred in your harness, load it by name via tool
  search first (e.g. `select:mcp__codegraph__codegraph_explore`).

- **Shell** (always works, no MCP needed):

  ```bash
  codegraph explore "<symbol names or question>"
  codegraph node <SymbolName>      # one symbol + caller/callee trail
  codegraph query "<search>"       # symbol search
  codegraph status                 # index health
  ```

### How to query well

- Ask a **natural-language question** ("how does X work", "where is Y handled")
  or throw a **bag of symbol/file names** at it — both work.
- Name a file or symbol to get its **current, line-numbered source**, byte-for-byte
  what `Read` returns and safe to `Edit` against. **Do not re-Read a file
  CodeGraph already printed.**
- One call usually answers the whole question. If you are on your third
  exploratory call, the query is too vague — name concrete symbols.
- **Don't delegate the lookup to a file-reading sub-agent.** CodeGraph *is* the
  pre-built index; a grep/read loop repeats work already done and costs far more
  for the same answer.

### Notes

- In Claude Code a `UserPromptSubmit` hook (`codegraph prompt-hook`, in
  `.claude/settings.json`) already injects structural context for each prompt.
  Treat source it returns as **already read**.
- `.codegraph/` is gitignored — indexing is each developer's local decision. If
  the directory is absent, skip CodeGraph entirely and fall back to grep/Read.
- The index lags writes by about a second; right after your own edit, trust the
  file, not the graph.

---

## 3. Language rules

This trips up every new agent, so it is explicit:

- **English, always**: `README.md`, everything under `docs/**`, ADRs, Mermaid
  diagrams, `schema.dbml`, `openapi.yaml`, module READMEs, **all code comments**,
  commit messages, PR titles/descriptions, CI job and check names.
- **Multilingual (product content, do not translate to English)**:
  `web/i18n/locales/{es,en,ca}.json` and `android/app/src/main/res/values*/strings.xml`.
  The web default locale is `es`; Android's default `values/` is Spanish, with
  `values-en/` and `values-ca/` overrides.
- **Conversation with the user** happens in whatever language they write in
  (usually Spanish). That does **not** change what you write into files.
- If you edit a file that still contains Spanish documentation or comments,
  **translate it as part of that change** instead of leaving it mixed.

Adding a user-facing string means adding the key to **every** locale — CI fails
otherwise (§7).

---

## 4. The 4 sources of truth

Before assuming how something works, consult the corresponding document — not
another module's code by analogy, and not memory from a previous conversation.
Full detail in [`docs/architecture/ARCHITECTURE.md`](docs/architecture/ARCHITECTURE.md).

| # | Source | Location | Governs |
|---|---|---|---|
| 1 | **DBML** | [`docs/database/schema.dbml`](docs/database/schema.dbml) | Tables, types, relations, indexes |
| 2 | **OpenAPI 3.1** | [`docs/api/openapi.yaml`](docs/api/openapi.yaml) | The single backend ↔ Android/Web REST contract |
| 3 | **Mermaid** | `docs/diagrams/`, `docs/architecture/` | Flows, state machines, navigation |
| 4 | **ADRs** | [`docs/decisions/`](docs/decisions/) | Technical decisions and their rationale |

**Order of operations, not optional:**

- Changing how backend and clients communicate → **`openapi.yaml` first**, then the handler.
- Changing the data model → **`schema.dbml` first**, then the goose migration in
  `backend/migrations/`, then `sqlc generate`. CI diffs the DBML against the
  migrations in a real Postgres and will catch drift.
- Making a non-trivial technical choice (a library, a pattern, a data structure
  that wasn't already defined) → add an ADR, numbered sequentially
  (`00NN-kebab-case-title.md`), following the shape of the existing ones:
  `# ADR-00NN: Title`, `**Status:**`, `## Context`, `## Decision`, `## Consequences`.
  Next free number: check `ls docs/decisions/`.

---

## 5. Architecture rules per area

Follow the established layering — do not introduce a new pattern in one slice.
The file-by-file map of each app is in
[`docs/architecture/PROJECT-STRUCTURE.md`](docs/architecture/PROJECT-STRUCTURE.md).

### Backend (Go) — modular monolith

- One package per feature under `internal/` (`auth`, `users`, `decks`, `games`,
  `game-actions`, `playgroups`, `statistics`, `tournaments`, `friends`, `sync`,
  `websocket`, `common`), each a self-contained vertical slice: `handler.go`,
  `service.go`, `db.go`, `dto.go`/`models.go`, `query.sql`.
- **Handler → Service → Repository.** The real decoupling boundary is
  Handler ↔ Service, *not* Service ↔ persistence: `Service` deliberately takes
  `*pgxpool.Pool` and works with sqlc/pgtype types. That is a consistent
  trade-off, not a leak — don't "fix" it by wrapping `Querier`.
- Repositories are **generated**: edit `query.sql`, run `make generate-sql`,
  commit the generated `*.sql.go`. Never hand-edit generated files — CI verifies
  `sqlc generate` leaves no diff.
- Errors go through `internal/common/errors.go` (`DomainError`/`MapError`).
  Never leak raw internal errors to clients.

### Android — MVVM + UDF

- `presentation/` (Compose + ViewModels) → `domain/` (use cases + repository
  interfaces) → `data/` (Room, Retrofit, `SessionManager`).
- Most ViewModels depend on `domain/`. The one deliberate exception is auth
  (`LoginViewModel`, `RegisterViewModel` and `SettingsViewModel` inject
  `AuthApi`/`CommanderApi` directly) — deliberate, see TASKS.md Stage 4.
- `data/repository/` decides what lives in Room vs. what hits the backend; it is
  not a pass-through.
- Remember the 2-second rule: anything on the in-game path must stay instant.

### Web — Nuxt SSR + Nitro BFF

- `server/` (Nitro) is the **only** place that touches session cookies
  (`httpOnly`) and the only thing that talks to the Go API. The browser never
  sees a token and never calls the Go API directly.
- `app/pages/`, `app/composables/` (`useAuth`, `useDecks`, `useStatistics`, each
  wrapping its slice of the REST contract), `app/middleware/auth.global.ts`.
- See [ADR-0004](docs/decisions/0004-web-client-nuxt.md).

---

## 6. Commands

**Full stack** (root, needs Docker):

```bash
docker compose up --build     # db + api + web
# first run: apply migrations by hand, they don't run inside the container
```

**Backend** (`cd backend`):

```bash
make run                 # run the API locally
make test                # go test -v -race -p 1 ./...  (some tests need Postgres, see internal/testutil)
make lint                # golangci-lint (pinned v2.12.2)
make lint-docker         # same, via Docker, if the binary isn't installed
make generate-sql        # sqlc generate — after ANY change to query.sql
make generate-sql-docker # same, via Docker
make migrate-up          # goose migrations up (migrate-down to roll back)
```

**Web** (`cd web`):

```bash
npm install
npm run dev              # localhost:3000, needs the API running separately
npm run lint             # eslint (CI runs it with --max-warnings=0)
npm run typecheck        # vue-tsc
npm run build            # nuxt build (SSR) — the check that actually catches SSR breakage
```

**Android** (`cd android`, **JDK 21 required** — Gradle 9.5 / AGP 9.3):

```bash
./gradlew assembleDebug
./gradlew testDebugUnitTest
./gradlew lintDebug
./gradlew connectedAndroidTest   # instrumented, needs a device/emulator
```

---

## 7. Quality gates

### Local pre-push hook

`.githooks/pre-push` reproduces the CI static checks in Docker before a push
(gofmt, go vet, golangci-lint, sqlc-no-diff, hadolint, eslint, typecheck, nuxt
build). It is **not active on a fresh clone**:

```bash
git config core.hooksPath .githooks
```

It skips itself when neither `backend/` nor `web/` changed (`.md`-only changes
don't count), and when Docker isn't available. `git push --no-verify` skips it
deliberately — only for a genuine one-off.

### GitHub Actions

Four pipelines, each with a `changes` job (dorny/paths-filter) so no check hangs
on a PR that doesn't touch its folder. **All check names are in English and are
the identifiers branch protection matches on — renaming one means updating the
protection settings in the same pass.**

- **`backend-ci.yml`** — gofmt/`go vet`; `golangci-lint`; `sqlc generate` leaves
  no diff; build + `go test -race` + goose migrations against a real Postgres;
  `hadolint` on `backend/Dockerfile`.
- **`android-ci.yml`** — Android Lint; `testDebugUnitTest`; `assembleDebug`;
  *string resources translated in every locale*.
- **`web-ci.yml`** — ESLint + `vue-tsc` + `nuxt build`; *i18n keys resolve in
  every locale*; `hadolint` on `web/Dockerfile`.
- **`docs-ci.yml`** — Spectral lint on `openapi.yaml`; `schema.dbml` compiles to
  SQL; **`schema.dbml matches the migrations`** (builds the schema twice in one
  Postgres — once with goose, once from the compiled DBML — and diffs
  `information_schema`). It also runs on `backend/migrations/**`, because a
  migration alone can invalidate the DBML.
- **`architecture-ci.yml`** — runs `.github/scripts/check-architecture.sh`: the
  layering and contract invariants, plus the README-hub rule. No path filter —
  the invariants are cross-cutting and the script is seconds of greps.
- **`labeler.yml`** — labels PRs by changed path (backend/web/android/docs).

**The i18n checks** (`.github/scripts/check-i18n-{web,android}.mjs`) exist
because a missing key fails nothing else: on web, Vue renders the raw key and
the page still returns 200; on Android the string silently falls back to
Spanish. Android resources deliberately left untranslated must carry
`tools:ignore="MissingTranslation"`.

**Branch protection caveat**: the required checks on `main` are 8, from
`backend-ci.yml`/`android-ci.yml`/`docs-ci.yml`. `web-ci.yml` was added
afterwards and its checks are not required yet; also, its `hadolint (Dockerfile)`
job shares a name with the backend one, so GitHub (which matches by job name)
lets either satisfy that check. Don't rely on web CI being blocking.

### Architecture guardrails

Four mechanisms enforce the layering, each owning the rules it can express
natively. **Nothing is checked twice** — if you add a rule, put it where it
belongs rather than next to a similar-looking one.

| Guardrail | Where | Owns | Runs in |
|---|---|---|---|
| `depguard` | [`backend/.golangci.yml`](backend/.golangci.yml) | a handler must not import a database driver | `golangci-lint` (required check) |
| Konsist | [`android/app/src/test/.../architecture/ArchitectureTest.kt`](android/app/src/test/java/com/commandercompanion/architecture/ArchitectureTest.kt) | Android layering, the enumerated auth exception, two ratchets on known debt | `testDebugUnitTest` (required check) |
| `eslint-plugin-boundaries` + `no-restricted-imports` | [`web/eslint.config.mjs`](web/eslint.config.mjs) | `app/` ↔ `server/` in both directions | `npm run lint` |
| `check-architecture.sh` | [`.github/scripts/check-architecture.sh`](.github/scripts/check-architecture.sh) | everything that is **not** an import | `architecture-ci.yml`, `pre-push` |

The script keeps what no import-level tool can state: SQL inside a string
literal, the web client reaching the Go API *by URL*, registered routes vs
`openapi.yaml`, markdown links, the date in `TASKS.md` — plus one import rule
that is genuinely inexpressible elsewhere: **a slice reaching into another
slice's sqlc `Queries`**, because `Service` and `Queries` live in the same Go
package, so no tool that reasons about import paths can tell them apart.

**Ratchets.** When you find a deviation too large to fix in the change that
found it, freeze it instead of waiving it: assert the exact set of offending
files, in a list that may only shrink. Two Konsist tests did this for the
Android layering debt; both are now plain invariants, because the debt was paid
off (see [ADR-0019](docs/decisions/0019-android-domain-owns-its-types.md)).
Adding a file to such a list to make the build pass is the one move that defeats
the point — fix the coupling, or say out loud that you are widening it.

### Definition of done

**Run this. It is the checklist, not a summary of it:**

```bash
sh .github/scripts/check-architecture.sh
```

It enforces, with the file and the fix named in each failure: handlers mapping
their errors, no slice reaching into another slice's `Queries`, SQL only in
`query.sql`, the web client not bypassing the Nitro BFF, every route present in
`openapi.yaml` (and no phantom paths), and every document linked from the
README hub. It also warns when `TASKS.md`'s review date has fallen behind the
code. `architecture-ci.yml` runs the same script on every PR, and
`.githooks/pre-push` runs it before a push, so a failure here is a failure
there. It is one of four guardrails — see the table above for what the other
three own; run `make lint` (depguard), `./gradlew testDebugUnitTest` (Konsist)
and `npm run lint` (boundaries) for those.

Then the parts a script cannot judge:

- [ ] Code works, not just compiles (no stub left returning dummy data).
- [ ] Relevant gate passes locally: `make lint && make test` / `npm run lint && npm run typecheck && npm run build` / `./gradlew lintDebug testDebugUnitTest`.
- [ ] New user-facing strings added to **all** locales.
- [ ] `TASKS.md` updated for what you actually did; `DECISIONS-LOG.md` entry if there's narrative worth keeping.
- [ ] A non-trivial technical decision recorded as an ADR (copy [`docs/decisions/TEMPLATE.md`](docs/decisions/TEMPLATE.md)).

Why the split: every rule in this repo that had an automated check was at 100%
compliance in the 2026-09-03 audit, while three that lived only in prose had
silently drifted — and the prose was clear. Nothing an agent has to *remember*
survives a cold session. So the mechanical rules moved into the script, and
what stays here is only what needs judgement.

---

## 8. Git workflow

- **Never commit to `main`.** Branch first, prefix by intent, matching existing
  history: `feat/`, `fix/`, `chore/`, `ci/`, `docs/` (e.g.
  `ci/dbml-schema-drift-check`, `fix/games-playgroup-n-plus-one`).
- **Commit subjects: imperative English, no conventional-commit prefix.**
  "Batch the seat lookup in ListGamesForPlaygroup", not "fix(games): ...".
- Merge into `main` **through a PR** — the repo history is PR merges.
- Only commit or push when the user asks for it.
- **Line endings are decided by `.gitattributes`** (`* text=auto eol=lf`), not by
  each clone's `core.autocrlf`. Generated code comes out of Linux containers as
  LF; a CRLF working tree used to make `sqlc generate` leave ~29 phantom-modified
  files. Don't add a `.gitattributes` override or flip `core.autocrlf`. `.bat`/
  `.cmd` stay CRLF on purpose.
- Dependabot runs weekly on gomod/npm/gradle/github-actions, **major updates
  only** (minor/patch are ignored deliberately). `typescript` majors are pinned
  back until `vue-tsc` supports TS 7.

---

## 9. Keeping the roadmap honest

Three files, three jobs — don't blur them:

| File | Job |
|---|---|
| [`ROADMAP.md`](docs/roadmap/ROADMAP.md) | Vision and stages — the *what* and *why*. Rarely edited. |
| [`TASKS.md`](docs/roadmap/TASKS.md) | Status, audited against the code. **Deliberately compact: one short line per item** (status + what + file pointer). |
| [`DECISIONS-LOG.md`](docs/roadmap/DECISIONS-LOG.md) | The narrative: why, gotchas, how it was verified, dates, PRs. Read only the entries you need — never the whole file up front. |

Rules:

- `TASKS.md` is updated **in the same change** that resolves the task.
- `- [x]` only when functionally complete. Half-done stays `- [ ]` with a short
  parenthetical saying what's missing.
- **Never delete completed tasks** — they are the progress history. If an
  approach is dropped, strike it through with the reason.
- If an item needs more than a sentence or two, put it in `DECISIONS-LOG.md`
  under the matching stage and link to it from the TASKS.md line. That split is
  the whole point: a new session shouldn't have to load the project's history
  just to check status.
- Discovered a task nobody listed? Add it to its stage section instead of
  leaving it loose in the conversation.
- At the end of a work session, update the `**Last reviewed:**` line and add the
  corresponding `DECISIONS-LOG.md` entry.
- Every new document under `docs/` (or a new module README) gets linked from
  **README.md §8**, the documentation hub, in the same change.

---

## 10. Per-tool agent configuration

These files are **gitignored on purpose** — they are local, per-developer setup,
not shared repo policy. This `AGENTS.md` is the shared part.

| File | Tool | Contents |
|---|---|---|
| `.mcp.json` | Claude Code | Registers `codegraph serve --mcp` as a stdio MCP server |
| `.claude/CLAUDE.md` | Claude Code | Short pointer: CodeGraph usage + the English-docs rule |
| `.claude/settings.json` | Claude Code | Permission allowlist + the `codegraph prompt-hook` UserPromptSubmit hook |
| `.claude/settings.local.json` | Claude Code | Machine-local permissions, never shared |
| `.gemini/settings.json` | Gemini CLI | Same CodeGraph MCP server |
| `GEMINI.md` | Gemini CLI | Same instructions as `.claude/CLAUDE.md` |

If you extend the rules in one of them, mirror the change here — this file is
what a *new* tool will read.

---

## 11. Environment gotchas

- **Development happens on Windows** with Git Bash / PowerShell available.
  Prefer POSIX shell syntax in scripts; `.githooks/pre-push` documents the MSYS
  path-rewriting workarounds (`MSYS_NO_PATHCONV=1`, `pwd -W`, stripping CR
  before gofmt) — reuse them rather than rediscovering them.
- **Docker is required** for the pre-push hook, `docker compose`, and the
  `*-docker` Make targets.
- **Backend tests are partly integration tests** (`backend/internal/testutil`)
  and need a Postgres; `make test` runs with `-p 1` for that reason.
- **Android needs JDK 21.** If the system `java` is older, point `JAVA_HOME` at
  a compatible JDK (Android Studio bundles one at `Android Studio/jbr`).
- **Google Sign-In is not wired to real credentials yet.** The backend answers
  `501` without `GOOGLE_CLIENT_ID`; Android's `GOOGLE_WEB_CLIENT_ID` is a
  placeholder. Creating the Google Cloud OAuth credentials is an external manual
  step, not a code task.
