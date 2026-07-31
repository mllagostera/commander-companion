# ADR-0007: PostgreSQL as the primary database

**Status:** Accepted and implemented — **inherited decision, context
reconstructed**. Like ADR-0006, this decision predates the project's ADR
history and is documented retroactively (2026-07-27) based on the actual
state of the code (`backend/migrations/`, `docs/database/schema.dbml`,
`backend/go.mod`), not from a discussion witnessed at the time.

## Context

The data model (Stage 2 of the ROADMAP) needs well-defined relationships
from day one: users, decks, games (`games`), game players (`game_players`),
game actions (`game_actions`), play groups (`playgroups`), and statistics
summary tables (`user_statistics_summary`, `deck_statistics_summary`). The
ROADMAP fixes PostgreSQL explicitly in the Stage 1 stack and in both
versions of the architecture diagram (`ROADMAP.md`, both Mermaid diagrams
end in `PostgreSQL`).

## Decision

**PostgreSQL** as the sole database engine, accessed via
`github.com/jackc/pgx/v5` (driver + `pgxpool.Pool` pool) from Go, with
`sqlc` generating typed data-access code over that driver (see ADR-0008)
and `goose` managing versioned migrations (`migrations/00001_initial
_schema.sql`, `00002_auth.sql`).

## Alternatives considered

- **MySQL/MariaDB**: equally viable for a conventional relational model;
  discarded in favor of Postgres, likely due to features the schema
  already leverages or is expected to leverage: expressive `CHECK`
  constraints (`CHECK (password_hash IS NOT NULL OR google_id IS NOT
  NULL)` on `users`, see `migrations/00002_auth.sql`), richer native types
  (UUID, JSON/JSONB — used in `game_actions.payload` per
  `game-actions/service.go`, which serializes/deserializes each action's
  payload as JSON), and the Go ecosystem's tooling (`pgx` is the de facto
  most mature and performant driver in the modern Go ecosystem, more so
  than its MySQL equivalents).
- **NoSQL database (MongoDB, DynamoDB)**: discarded because the domain is
  intrinsically relational (users ↔ decks ↔ games ↔ game players ↔ game
  actions, with real referential integrity: a `game_player` can't exist
  without its `game` and its `deck`) and because aggregated statistics
  (Stage 7) benefit from SQL `GROUP BY`/aggregations rather than
  map-reduce or document aggregation.
- **SQLite**: discarded for not fitting "synchronization between players"
  and "scalable architecture" (explicit ROADMAP goals) — the backend is a
  centralized server with multiple concurrent clients, not a single-user
  app with embedded storage (that role is filled by Room, but *on
  Android*, not in the backend — see ADR-0009).

## Consequences

- The schema lives in `docs/database/schema.dbml` as the source of truth
  (one of the "four sources of truth" declared in `ROADMAP.md`), and is
  validated in CI by compiling to SQL with `@dbml/cli` (`docs-ci.yml`) —
  committing to Postgres specifically (not "generic SQL") is already
  reflected in the use of Postgres-specific types and syntax in the
  migrations and in the queries. Migrating engines at this point would mean
  rewriting the DBML, migrations, and `sqlc` queries.
- All backend integration testing (`internal/testutil`, used by `auth`,
  `decks`, `games`, `game-actions`, `playgroups`, `statistics`) runs
  against **real Postgres**, not a mock or an in-memory SQLite — more
  faithful to production, but requires a Postgres instance available
  (locally or in CI, see `backend-ci.yml`) to run the tests, and forces
  `go test -p 1` because the tests share the same database and `TRUNCATE`
  each other.
- The statistics engine (Stage 7) and the future Match Engine (see the
  second diagram in `ROADMAP.md`) assume they can read/write against the
  same Postgres instance without a replication or sharding layer —
  acceptable as long as the project remains a single-maintainer modular
  monolith (see ADR-0010).
- The Postgres major version is fixed in two places that must be kept in
  sync manually (there's no single source of truth for this): the `db`
  service image in `docker-compose.yml` and the `postgres` service image
  in `backend-ci.yml` (2026-07-27: both updated to **18**,
  `postgres:18-alpine`; previously they were misaligned, `15-alpine` and
  `16-alpine` respectively). A major version jump isn't compatible with
  the data volume of a previous version (different on-disk format) and,
  from image 18+ onward, isn't compatible with the previous mount layout
  either (`/var/lib/postgresql/data`): it requires recreating the dev
  volume or migrating with `pg_upgrade`, and adjusting the mount to
  `/var/lib/postgresql`.

## References

- `docs/database/schema.dbml`
- `backend/migrations/00001_initial_schema.sql`, `00002_auth.sql`
- `backend/go.mod` (`github.com/jackc/pgx/v5`)
- `docs/roadmap/ROADMAP.md`, "Sources of truth" section and both
  architecture diagrams
