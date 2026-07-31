# ADR-0011: Migration naming strategy and recompute of pre-calculated statistics

**Status:** Accepted (2026-07-27) — formalizes a convention that had already
been followed de facto since `migrations/00001_initial_schema.sql`, and proposes
(without implementing it yet) a backfill mechanism that does not exist today.

## Context

The project already has 6 migrations (`00001` to `00006`, see ADR-0008 for the
choice of goose+sqlc) and two pre-calculated statistics tables
(`user_statistics_summary`, `deck_statistics_summary`) that get updated
incrementally every time a game finishes
(`internal/statistics/service.go: RecalculateForGame`, see Stage 1 and 7 of
`docs/roadmap/TASKS.md`). Two gaps remained undocumented:

1. **Migration naming**: the pattern used (`%05d_slug_en_snake_case.sql`)
   was never written down anywhere — anyone adding a new migration
   has to infer it by reading the directory.
2. **Statistics recompute**: if the aggregation formula in
   `RecalculateForGame` changes (for example, to stop treating
   `CombatDamage`/`CommanderDamage` the same way, see the known limitation
   in `TASKS.md:70`), there is no mechanism to recalculate the summary
   rows for games that have already finished.
   `docs/database/schema.dbml` (line ~102) already leaves a note acknowledging
   this need, without proposing how.

## Decision

### 1. Migration naming

The convention already used de facto across the 6 existing
migrations is formalized in writing:

- **File name**: `%05d_slug_en_snake_case.sql` (5 zero-padded digits,
  underscore, short description in snake_case). Real examples:
  `00002_auth.sql`, `00004_status_constraints.sql`,
  `00006_deck_image_url.sql`.
- **Single flat sequence**: all migrations share a single correlative
  numbering under `backend/migrations/`, with no prefix or
  per-module sub-folder, even though a given migration typically affects only one
  `internal/` module. This makes it easy to know the order in which they were
  applied without having to look across several folders.
- **Mandatory structure**: each migration wraps its `-- +goose Up` and
  `-- +goose Down` in its own `-- +goose StatementBegin` /
  `-- +goose StatementEnd` block (even for a single statement), and `Down` reverses
  `Up` in exactly the opposite order (LIFO for migrations with more than one
  statement — see `00001_initial_schema.sql`, which drops the 9 tables in
  the reverse order of creation due to FKs).
- **Comments that cite real code**: when a migration enforces something the
  Go code already validates at runtime (a `CHECK`, an index designed for
  a specific query), the comment in the `.sql` file references the corresponding
  Go file/function (see `00004_status_constraints.sql`, which cites
  `internal/games/service.go` and `internal/game-actions/service.go`).
- **Verification before merging**: run `up` → `down` → `up` against a real
  Postgres instance locally before opening the PR (this was already the practice
  followed, loosely documented across `TASKS.md`'s history, never in one place).
  `backend-ci.yml` only runs `up` against the CI Postgres service — the
  `down`→`up` cycle remains a manual check, not automated.
- **Changing the schema still follows the order already established in ADR-0008**:
  `docs/database/schema.dbml` → goose migration → the affected module's `query.sql`
  if used columns change → `sqlc generate`.

### 2. Recompute of pre-calculated statistics

Today there is no backfill mechanism at all (confirmed: no CLI, endpoint,
or script — the only way to touch `user_statistics_summary`/
`deck_statistics_summary` is `RecalculateForGame`, called exactly once per
game from `games.FinishGame`). It is proposed, for when historical
statistics need to be re-derived:

- A new one-off command, `backend/cmd/recalculate-stats/main.go`, that:
  1. `TRUNCATE`s the two summary tables.
  2. Iterates over every game with `status = 'finished'` **in chronological
     order** (`created_at` or `id`, either works as long as it's a total,
     deterministic order).
  3. Calls `statistics.RecalculateForGame(gameID)` once per game.
- **Invariant the script must respect**: `RecalculateForGame`'s upserts are
  incremental (`ON CONFLICT DO UPDATE SET x = x +
  EXCLUDED.x`), not replacements — calling it twice for the same game
  duplicates its contributions. This is not currently an active bug because the
  `games` state machine doesn't allow finishing the same game twice, but
  any future backfill script **must guarantee that each
  game is processed exactly once** (hence the initial `TRUNCATE`: it is
  simpler to recalculate everything from scratch than to attempt a partial,
  idempotent backfill).
- This command is not implemented in this pass — it's a proposal for
  when the aggregation formula actually changes and history needs to be
  re-derived; until then, documenting it is enough.

## Alternatives considered

- **Timestamp-based migrations instead of a sequence** (`20260727120000_x.sql`,
  a common pattern in Rails/Django): avoids number collisions if two branches
  create a migration in parallel, but the project already has 6 migrations
  with the sequential scheme and has a single maintainer (ADR-0010) — the collision
  risk the timestamp solves doesn't apply today. Switching schemes midway
  would create more confusion than the problem it solves.
- **Incremental snapshot instead of full recompute** (storing a
  checkpoint of which games have already been processed and only recalculating new
  ones): more efficient for large databases, but solves a scale problem
  the project doesn't have yet (iterating over all finished games of a
  project still in active development is cheap) and complicates the idempotency
  logic right at the point where it's currently simplest to guarantee it
  (recalculating everything from scratch). The simple option is preferred until
  real volume justifies otherwise.
- **Making `RecalculateForGame` idempotent on its own** (with an
  `recalculated_games` audit table or similar) instead of delegating the
  invariant to the backfill script: ruled out for now because it adds a
  table and an extra check to the hot path (end of every game) for a
  case that only matters on the cold path (manual backfill, infrequent).

## Consequences

- Any new migration must follow the naming and structure from this
  ADR; a reviewer who sees a name outside the pattern (date, sub-folder,
  a `Down` that doesn't reverse `Up` in the opposite order) can flag it citing this
  document.
- The `recalculate-stats` command remains a concrete future task (not yet
  opened in `TASKS.md` as a code item, only documented here) —
  it gets opened once an aggregation formula actually changes and history needs to
  be re-derived.
- **Follow-up note, outside the scope of this ADR**: `docs-ci.yml`'s
  `dbml-validate` only verifies that `schema.dbml` compiles to SQL
  (`dbml2sql`), it does not compare it against the actually migrated schema — DBML and
  migrations can diverge without CI detecting it. Closing that gap
  (comparing `dbml2sql` against a `pg_dump --schema-only` of a freshly
  migrated database) remains a future improvement, not part of this change.

## References

- `backend/migrations/00001_initial_schema.sql` through `00006_deck_image_url.sql`
- `backend/internal/statistics/service.go` (`RecalculateForGame`)
- `docs/database/schema.dbml` (note about pre-calculated statistics)
- [ADR-0008](0008-sqlc-goose.md) (sqlc + goose, schema change order)
- `docs/roadmap/TASKS.md`, Stage 2 and Stage 7
