# Architecture and Design Principles

This document describes the architecture and the fundamental design principles of the **Commander Companion** project.

## The 4 Sources of Truth

To ensure the project can scale and that parallel development (including collaboration with AIs) is efficient and coherent, the system rests on four documentary pillars:

1. **DBML (Database Markup Language):**
   Defines the database schema, types, relationships, indexes, and constraints.
   Location: `docs/database/schema.dbml`

2. **OpenAPI 3.1:**
   The single, inviolable contract between the backend (Go) and the clients (Android and Web/Nuxt, see [ADR-0004](../decisions/0004-web-client-nuxt.md)). Any change in communication must be reflected here first.
   Location: `docs/api/openapi.yaml`

3. **Mermaid (Diagrams):**
   Used to document architecture, data flows, state machines, and system behavior.
   Location: Inside the Markdown files under `docs/architecture/` and `docs/diagrams/`.

4. **ADR (Architecture Decision Records):**
   Structured documentation of key technical decisions, context, options considered, and consequences.
   Location: `docs/decisions/`

---

## System Architecture

### Backend (Go)
- **Pattern:** Modular Monolith, one package per feature under `internal/`
  (`auth`, `decks`, `games`, `playgroups`, `statistics`, ...), each a
  self-contained vertical slice (`handler.go`, `service.go`, `db.go`,
  `dto.go`/`models.go`, `query.sql`).
- **Internal structure, per slice:**
  - `Handler`: transport layer (HTTP/REST, WebSocket) — the only layer that's
    actually decoupled from infrastructure. It knows nothing about SQL or the
    Postgres driver, only about `Service`.
  - `Service`: business logic. In practice it is **not** pure/infra-free — it
    takes `*pgxpool.Pool` directly (to open transactions) and works with the
    types sqlc generates (`pgtype.UUID`, `pgtype.Text`, `pgtype.Timestamptz`,
    `pgx.ErrNoRows`, etc.) rather than plain Go types. This is a deliberate,
    consistent trade-off across every slice, not an accidental leak: with a
    single Postgres database and no plan to swap it, wrapping every
    sqlc-generated `Querier` in a translation layer would buy testability we
    don't currently need, at the cost of touching every method signature in
    the backend. The real decoupling boundary in this codebase is
    Handler ↔ Service, not Service ↔ persistence.
  - `Repository`: persistence and data access — the `Querier` interface plus
    `.sql.go` code sqlc generates from `query.sql`, consumed directly by
    `Service`.

### Client (Android)
- **Pattern:** Clean Architecture + MVVM + UDF (Unidirectional Data Flow).
- **Logical modules:**
  - `Presentation`: UI with Jetpack Compose and ViewModels.
  - `Domain`: use cases and repository interfaces — a layer that doesn't
    exist yet; `ViewModel`s go straight against `data/repository/` or,
    in auth, straight against the API (see `docs/roadmap/TASKS.md`, Stage 4).
  - `Data`: repositories (`GameRepository`, `DeckRepository`) that decide
    what's persisted in Room (local) and what calls the real backend (Retrofit,
    `CommanderApi`) — not a purely pass-through layer, it already holds the
    logic for "what goes to which side".

### Web Client (Nuxt)
- **Pattern:** SSR with a BFF layer (Nitro) in between — see
  [ADR-0004](../decisions/0004-web-client-nuxt.md), `web/README.md`.
- **Logical modules:**
  - `server/` (Nitro): the only place that touches session cookies (`httpOnly`)
    and acts as an authenticated proxy to the Go API — the browser never sees a
    token nor calls the Go API directly.
  - `app/`: the actual Nuxt code — `pages/` (routes), `composables/`
    (`useAuth`, `useDecks`, `useStatistics`, each wrapping its slice of
    the REST contract), `middleware/` (gating of authenticated routes).
- 100% decoupled from Android: they share the REST contract
  (`docs/api/openapi.yaml`), not code or components.
