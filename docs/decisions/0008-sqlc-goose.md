# ADR-0008: sqlc for typed data access + goose for migrations

**Status:** Accepted and implemented — **inherited decision, context
reconstructed** (see the methodology note in ADR-0006 and ADR-0007; written
retroactively on 2026-07-27 based on `backend/sqlc.yaml`,
`backend/migrations/`, and the eight `internal/` modules that use
sqlc-generated code).

## Context

With Go + PostgreSQL already chosen (ADR-0006, ADR-0007), two tooling
decisions remained with direct impact on how each `internal/` module is
written: how to go from SQL to typed Go code (full ORM vs. query builder
vs. code generation from raw SQL), and how to version and apply schema
changes reproducibly across local/CI/production.

## Decision

- **sqlc** (`sqlc.yaml`, `sql_package: "pgx/v5"`) generates the data-access
  code for the six modules with their own queries (`users`, `auth`,
  `decks`, `playgroups`, `games`, `game-actions`, `statistics`) from SQL
  handwritten in each `internal/<module>/query.sql`, producing structs and
  a `Querier` interface (`emit_interface: true`) per module, directly
  compatible with `pgxpool.Pool` (`emit_json_tags: true` to serialize
  straight to response DTOs).
- **goose** manages versioned migrations
  (`migrations/00001_initial_schema.sql`, `00002_auth.sql`), applied in CI
  against a real Postgres (`backend-ci.yml`) as part of the build gate.

## Alternatives considered

- **Full ORM (GORM, ent)**: offers more "magic" (navigable relations,
  migrations auto-generated from structs), but at the cost of implicit SQL
  that's hard to predict/optimize and an additional abstraction layer over
  the driver. Discarded in favor of writing SQL by hand and generating only
  the typed *binding* — consistent with the ROADMAP's philosophy of "DBML
  as the source of truth for the schema" rather than deriving the schema
  from Go code.
- **Dynamic query builder (squirrel, goqu)**: intermediate between an ORM
  and raw SQL, but doesn't give static typing of returned columns without
  still writing the `Scan` by hand — sqlc gives that type safety by
  generating the binding directly from the real SQL and the real schema,
  catching at generation time (`sqlc generate`) columns that don't exist or
  types that don't match, before hitting runtime.
- **Manual migrations with no tool (loose scripts)**: discarded due to lack
  of traceability of what was migrated and when in each environment; goose
  solves this with its version table and symmetric `up`/`down` commands
  (used explicitly in CI: "goose migrations against real Postgres").
- **Another migration tool (golang-migrate, Atlas)**: golang-migrate is the
  closest alternative in popularity; goose was probably chosen for its
  migration syntax as plain SQL with `-- +goose Up`/`-- +goose Down`
  annotations in the same file (more readable in review than
  golang-migrate's separate `.up.sql`/`.down.sql` files).

## Consequences

- Changing a table's schema requires touching, in order: `docs/database/
  schema.dbml` (documentary source of truth) → a new goose migration → the
  affected module's `query.sql` if used columns change → `sqlc generate` to
  regenerate the binding. Skipping `sqlc generate` leaves the Go code out
  of sync with the real schema; that's why `backend-ci.yml` runs `sqlc
  generate` and fails the build if it detects pending diffs.
- The version of the `sqlc` image/binary matters: `TASKS.md`'s own history
  records that `sql_package: "pgx/v5"` had to be pinned in `sqlc.yaml`
  because the default generator produced a `DBTX` incompatible with
  `pgxpool.Pool` — a reminder that sqlc and the chosen driver (ADR-0007)
  must be kept aligned on every update.
- Every new module with its own data access repeats the same pattern: an
  entry in `sqlc.yaml` + `query.sql` + `sqlc generate` — it's mechanical,
  but it's manual work per module (there's no automatic generation of full
  CRUD like in a scaffolding ORM).
- Additional indexes beyond the PKs (mentioned as pending in `TASKS.md`,
  Stage 2) and constraints such as `games.status`/`action_type` as an enum
  instead of free `varchar` remain outside what sqlc/goose resolve
  automatically — still manual schema-design work.

## References

- `backend/sqlc.yaml`
- `backend/migrations/00001_initial_schema.sql`, `00002_auth.sql`
- `docs/roadmap/TASKS.md`, Stage 1 ("`sqlc.yaml` was fixed...") and
  Cross-cutting (`backend-ci.yml` CI)
