# ADR-0010: Modular monolith instead of microservices

**Status:** Accepted and implemented — **inherited decision, context
reconstructed** (see the method note in ADR-0006; written retroactively
on 2026-07-27 based on `backend/cmd/api/main.go` and the structure of
`backend/internal/`).

## Context

`docs/roadmap/ROADMAP.md` is explicit and early about this: "Initially it
will be a **Modular Monolith**. There will be no microservices." — it's one
of the few architectural decisions declared in the ROADMAP itself instead
of remaining implicit in the code. The project needed to organize
functionality clearly separable by domain (auth, users, decks,
games, game actions, playgroups, statistics, sync,
websocket) without necessarily paying, from day one, the operational cost
of deploying and coordinating independent services.

## Decision

**A single Go binary** (`cmd/api/main.go`) that registers all the domain
modules as independent packages under `internal/` (`auth`, `users`,
`decks`, `games`, `game-actions`, `playgroups`, `statistics`, `sync`,
`websocket`, `common`), each with its own `service.go` (business
logic), `handler.go` (Fiber routes), and `query.sql`/sqlc-generated code
(data access) where applicable — but all sharing the same process,
the same Postgres connection `pgxpool.Pool`, and the same deployment.
`ROADMAP.md` explicitly documents the expected evolution: a second
architecture diagram "in later phases" splits out an `API Gateway`, a
`Match Engine`, and a `Statistics Engine`, but as a future aspiration, not
as the initial design.

## Alternatives considered

- **Microservices from the start** (one service per domain: auth-service,
  games-service, statistics-service, etc.): explicitly ruled out by
  the ROADMAP. The coordination cost (service discovery, inter-service
  communication, independent deployments, transactional consistency
  between services — e.g. `FinishGame` needs to reliably trigger
  `RecalculateForGame`, something trivial as an in-process interface call
  today and much more costly as a network call with its own
  partial failure modes) is not justified for a single-maintainer
  team and a product still in the phase of defining its MVP.
- **Non-modular monolith** (everything in a single `main` package, without
  separation by domain): would have been faster to bootstrap, but much harder
  to maintain as modules grow — the separation into `internal/
  <domain>/` with explicit interfaces between modules (e.g.
  `games.StatisticsRecalculator`, the interface `games` uses to
  decouple itself from the concrete `statistics` implementation and be
  able to mock it in tests) already works as a clear modular boundary even
  within a single process, and is what would allow extracting a module into its
  own service later without redesigning its internal logic.
- **Extracting the Websocket/Match Engine as a separate service already**: it
  was decided not to bring this forward — the ROADMAP explicitly leaves it for
  "later phases." **Note (2026-07-27)**: `internal/websocket/` is no longer
  empty — the Stage 6 server is implemented
  ([ADR-0005](0005-websocket-protocol.md)) as one more module within
  the same monolith (an in-memory `Hub` in a single process), not as a separate
  service; this ADR's decision remains the same, only the state changed from
  "no code" to "implemented, but still inside the monolith."

## Consequences

- All modules share the same `pgxpool.Pool` and the same process
  lifecycle — a failure or a backend deployment affects *all*
  domains (auth, games, decks, etc.) at once; there is no fault
  isolation between modules the way an independently crashing
  microservice would provide.
- Scaling horizontally today means scaling the entire binary (more
  replicas of the same process behind a load balancer), not selectively
  scaling the module under the most load (e.g. `game-actions` during
  active games, which predictably receives more traffic than `playgroups`).
  Acceptable while real usage volume doesn't demand otherwise.
- Module boundaries (`internal/<domain>/`) are a code convention, not
  a deployment boundary enforced by infrastructure — nothing today prevents
  a module from importing another module's internal package directly instead
  of going through an explicit interface, other than code review discipline (a single
  maintainer, no mandatory PR approval process, see `TASKS.md`
  branch protection: "PR approval is not required").
- If the project needs real microservices in the future (the second
  diagram in `ROADMAP.md` already anticipates it), the most natural migration
  path is to extract `websocket`/Match Engine first (Stage 6, server already
  implemented — see [ADR-0005](0005-websocket-protocol.md), decoupled from
  `games`/`game-actions` via the `Broadcaster` interface, the same pattern as
  `StatisticsRecalculator`) and `statistics` (Stage 7, already has that
  decoupling interface ready to become an HTTP/gRPC client instead of an
  in-process call) — these are the two modules the ROADMAP itself already
  draws as separate future services.

## References

- `docs/roadmap/ROADMAP.md`, "General architecture" section (both
  Mermaid diagrams) and "Initially it will be a Modular Monolith"
- `backend/cmd/api/main.go`
- `backend/internal/` (module structure)
- `backend/internal/games/service.go` (`StatisticsRecalculator` interface
  as an example of an explicit modular boundary)
