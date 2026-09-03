# ADR-0019: The Android domain layer owns its types

**Status:** Accepted (2026-09-03)

## Context

`ARCHITECTURE.md` described the Android client as "Clean Architecture + MVVM +
UDF", with a `Domain` layer holding use cases and repository interfaces that
`Data` implements. The dependency graph did not match that description.

A Konsist test added while wiring up architecture guardrails made it visible:
eight files under `domain/` imported from `data/`. The domain's own repository
interfaces were declared in terms of Retrofit DTOs —
`DeckRepository.listDecks(): Result<List<DeckDto>>`,
`StatisticsRepository.userStats(): Result<UserStatsDto>` — and
`GameRepository.observeHistory()` returned `GameWithPlayers`, a Room relation
class carrying `@Embedded`/`@Relation`. The arrow pointed the wrong way: nothing
in `domain/` could be read, tested, or reasoned about without the data layer,
and a change to the wire format or the database schema reached the domain
directly. Sixteen files under `presentation/` consumed those same DTOs, so the
coupling ran all the way to the screens: `HistoryScreen` compared a persisted
`status` column against the string literal `"FINISHED"`.

This had never been decided, only accumulated. The layer names had been in place
long enough to be believed.

## Decision

**Move the payload types into `domain/model/` and let `data/` depend on them,
keeping the serialization annotations on the domain types.**

`Deck`, `Game`, `GamePlayer`, `GameAction`, `NewGameAction`, `Page`,
`Playgroup`, `PlaygroupMember`, `Friend` and the friend-request models, and the
statistics models all moved out of `data/remote/dto/` into `domain/model/`,
dropping the `Dto` suffix. They keep `@Serializable`/`@SerialName`, so Retrofit
deserializes straight into them. `GameSocketEvent` moved too, since
`GameRepository` exposes a `Flow` of it. Request bodies that only `data/` ever
names — `CreateDeckRequest`, `JoinGameRequest`, the auth bodies — stayed put.

Persistence types are the exception and are mapped for real: Room's
`GameWithPlayers` becomes `PlayedGame`/`PlayedSeat` in
`GameRepositoryImpl.observeHistory()`. The mapping also resolves the persisted
`status` string into an `isFinished` boolean, so no caller compares against
`"FINISHED"` any more.

### Why not a parallel set of DTOs plus mappers

The textbook alternative — pure annotation-free domain models, DTOs left in
`data/`, and `toDomain()` mappers at the boundary — buys genuine isolation: the
domain would not know that Retrofit or kotlinx.serialization exist, and a
renamed JSON field would stop at the mapper.

It was rejected as disproportionate here, for the same reason the backend
rejected the equivalent move. `ARCHITECTURE.md` already argues that wrapping
every sqlc-generated `Querier` in a translation layer "would buy testability we
don't currently need, at the cost of touching every method signature in the
backend". The Android situation is the same shape: one backend, one
serialization library, no second consumer of these types, and roughly twenty
model types that would each need a twin and a mapper to maintain in lockstep.
Choosing differently on the two sides of the same project would have been the
inconsistency, not the fix.

## Consequences

- `domain/` no longer imports `data/` anywhere. The layering `ARCHITECTURE.md`
  claims is now true, and a Konsist test asserts it on every CI run rather than
  leaving it to be rediscovered.
- `presentation/` consumes domain models. The only files still importing data
  types are the three auth screens, which by an earlier decision sit outside the
  domain layer (`TASKS.md`, Stage 4) — enumerated in the test, not waived.
- **The remaining coupling is to the REST contract, not to the data layer.** A
  renamed field in `openapi.yaml` still lands on a domain model's
  `@SerialName`. That is the accepted price. It is bounded: the contract is
  itself a source of truth in this repo (README §3), and changing it already
  requires a deliberate, documented edit.
- Domain models carry serialization annotations, so `domain/` depends on
  `kotlinx.serialization`. It remains free of Android, Retrofit and Room, which
  is what keeps use cases unit-testable on the JVM.
- If a second consumer of these shapes ever appears, or the client needs to
  present data in a form the API does not return, this decision should be
  revisited — that is the point at which real mappers start paying for
  themselves.
