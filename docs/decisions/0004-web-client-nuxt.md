# ADR-0004: Second web client with Nuxt 4 + Tailwind, decoupled from the backend

**Status:** Accepted, skeleton started (2026-07-26, updated 2026-07-27)

## Context

The original roadmap only planned for a native Android client. The need
arose for two features for which a web client is more appropriate than
adding more screens to the mobile app meant for quick input during a game:
importing decks from Moxfield (a "paste a URL and go" flow, more
comfortable on desktop) and viewing post-game statistics (data
visualization, also more comfortable on large screens).

## Decision

- Framework: **Nuxt 4 + Tailwind CSS**. The latest available major version
  was chosen instead of settling on Nuxt 3 — a general project criterion:
  start every new dependency on its most up-to-date version rather than an
  earlier "more proven" one, unless there's a concrete reason not to. Nuxt
  4 mainly changes the folder structure convention (all app code lives
  under `app/`) relative to Nuxt 3; it doesn't affect any of the other
  decisions in this ADR.
- **100% decoupled from the backend**: it only consumes the REST API over
  HTTP (the same API Android uses), with no shared logic or deployment
  coupling. This was already enabled by [ADR-0003](0003-permissive-cors-in-dev.md)
  (CORS) — without that, a frontend on another origin couldn't call the API
  from the browser.
- Rendering mode: **SSR** (full Nuxt, with Nitro running as a Node process
  at runtime) — not a static SPA. Unlike `tools/auth-test/` (a single
  static HTML file, with no server of its own), this client does need its
  own deployed server process.
- Location in the repo: `web/`, at the same level as `android/` and
  `backend/`.
- Package manager: **npm** (already the tool used by the existing CI
  workflows for Spectral and `@dbml/cli` — zero new tooling to
  install/learn in the pipeline).
- **Original order of work**: first the real backend for what this client
  is going to show, then the Nuxt scaffolding. Scaffolding in parallel with
  mocked data was considered, but discarded to avoid duplicating work
  (mock → replace with real). **Partially moved up on 2026-07-27** at the
  user's explicit request: the auth skeleton (email/password + Google
  login) was scaffolded already, because that flow doesn't depend on the
  game/statistics engine still pending in the backend. The rest (Moxfield
  import, statistics) is still waiting on that backend work before being
  built.

## Alternatives considered

- **Nuxt in SPA mode (`ssr: false`)**: compiles to static assets, with no
  Node process at runtime, deployable to any CDN — lighter and more in
  keeping with "super lightweight". It was the initial recommendation, but
  the user explicitly preferred **full SSR** when deciding.
- **Extending the Android client** instead of adding a web client:
  discarded because the use case (importing a deck by pasting a URL,
  reviewing statistics charts) fits desktop/browser better than the game
  app, which is optimized for speed during play, not for
  management/lookup tasks.
- **pnpm** instead of npm: faster/lighter, but priority was given to not
  introducing a new tool into the already-existing CI toolchain.
- **Nuxt 3** (the version originally decided on 2026-07-26): changed to
  Nuxt 4 the next day because the project's criterion is to start with the
  most up-to-date major version of each new library, not the earlier more
  proven one — there was no technical issue with Nuxt 3, it was purely
  about aligning the decision with that criterion before more code was
  built on top of it.

## Consequences

- The Nuxt client needs its own deployed process (unlike a static SPA).
  **Resolved (2026-07-27)**: `web/Dockerfile` (multi-stage Node 24 build,
  runs `node .output/server/index.mjs`) + `docker-compose.yml` centralized
  at the repo root (previously lived in `backend/`), with `db`/`api`/`web`
  services. Verified end-to-end: `docker compose up --build`, migrate,
  register a user, login, and authenticated SSR render working against the
  real API within the Compose network.
  - Real gotcha found and fixed: SSR calls (e.g. `GET /auth/me` when
    loading `/`) run *inside* the `web` container, where `localhost:8080`
    doesn't resolve to the API — `NUXT_API_BASE` (internal hostname
    `http://api:8080/api/v1`, server only) was separated from
    `NUXT_PUBLIC_API_BASE` (browser, `http://localhost:8080/api/v1`).
  - Real gotcha found and fixed: `api` wasn't waiting for Postgres to be
    ready (`depends_on` with no condition) and crashed on a cold start — a
    healthcheck (`pg_isready`) + `condition: service_healthy` was added.
  - Real production deployment (beyond testing the stack locally) is still
    undecided: where to deploy these containers, TLS, domain, etc.
- Initial skeleton already created in `web/` (2026-07-27): Nuxt 4 +
  Tailwind + login (email/password + Google) + a minimal protected screen.
  **Completed the same day**: user registration (`register.vue`), tokens
  in `httpOnly` cookies via Nitro (BFF) with automatic refresh, Moxfield
  import (`decks.vue`, with a thumbnail of the commander's art crop and an
  updatable image via `POST /sync/moxfield`), and statistics
  (`statistics.vue`, global and per-deck) — see `docs/roadmap/TASKS.md`,
  Stage 4b, for the full piece-by-piece detail. `web-ci.yml` (ESLint +
  typecheck + `nuxt build` + hadolint) added the same day.

## References

- `docs/roadmap/TASKS.md`, "Stage 4b: Web Client (Nuxt)" section
- [ADR-0003](0003-permissive-cors-in-dev.md) (CORS, technical prerequisite)
