# ADR-0016: Standalone Swiss-format Commander tournaments

**Status:** Accepted (2026-08-09)

## Context

The user asked to be able to generate Commander tournaments from the web
client: define a player roster, run it as a Swiss-format event with 3-4
player pods per table (not 1v1 — Commander is a multiplayer format), score
2/1/0/0 by finish position, and hand out a join code that a participant can
use to find their assigned table each round. This was Stage 9 in the roadmap
("Social/tournaments," deliberately left undesigned) but was pulled forward
as a standalone feature ahead of the rest of that stage.

Several scope questions were resolved with the user before implementation:

- **Participants**: registered app users *and* guests with no account (name
  + commander only) — the organizer can't otherwise run a real in-person
  event where not everyone has installed the app.
- **Round count**: computed automatically from the final roster size, not
  organizer-set — one less decision for the organizer to get wrong.
- **Scope**: standalone, not tied to an existing `playgroups` group — any
  authenticated user can create one from scratch, which also fits guest
  participants (a playgroup-anchored design would have forced guests to
  somehow join a group they have no account to be a member of).
- **Android**: out of scope for this pass — backend + web only. The API is
  shaped so a future Android screen can call it.
- **"Entering the match" via the code**: the literal ask ("podrás poner el
  código en la app para ingresar en la partida donde los sitios estarán
  decididos por el torneo") reads as wanting the code to land the player in
  a live, seated game. Before committing to that, the existing
  `games`/`game_players` schema and ADR-0013's proxy-join authorization were
  checked against it — see Decision below for why that path was rejected for
  v1 in favor of a read-only lookup.

## Decision

### Self-contained schema, not wired into `games`/`game_players`

Five new tables (migration `00016_tournaments.sql`): `tournaments`,
`tournament_participants`, `tournament_rounds`, `tournament_tables`,
`tournament_table_seats`. None of them reference or are referenced by
`games`/`game_players` — a tournament table's "match" exists only within
this new schema. The organizer records each table's finish order manually
in the web UI once it's done playing; that's what drives scoring and the
next round's pairings.

Two things made wiring a table straight into the live life-tracker
(`games`/`game_players`/WebSocket) impractical for this pass:

1. **Guests have no seat representation there.** `game_players.user_id`
   points at `users(id)` with no concept of an accountless participant — a
   guest can't be given a real-time-tracked seat without first inventing
   that concept in a schema several other features (statistics, proxy-join)
   already depend on having a real user behind every seat.
2. **ADR-0013's proxy-join authorization doesn't cover this caller.** Today
   an organizer can only seat someone else in a game if they share a
   `playgroup_id` with them (`authorizeProxyJoin` in `internal/games/service.go`).
   A tournament organizer and a participant who joined by a code shared at
   an event usually share no playgroup at all.

Both are solvable, but each is its own design problem, not a detail to
absorb silently into a feature whose actual request was "generate
tournaments from the web." `GET /tournaments/lookup?code=` gives a
participant their current round's table number, seatmates, and commanders —
enough to know where to sit — without depending on either fix. Live-tracker
integration (for tables where every seat is an app user, at least) is a
natural, distinct fast-follow.

### Self-service registration, not organizer-driven

`POST /tournaments/join` (join code + `deck_id`) is called by the
participant themselves, authenticated — not by the organizer picking on
their behalf. This is what makes "si son usuarios de la aplicación,
cargará sus decks para que puedan escogerlo" true: the decks that load are
the *caller's own* (`internal/decks.Service.GetDeck`'s existing ownership
check, reused via a small `DeckLookup` interface — same pattern as
`moxfieldimport.DeckImporter`), because the caller is the participant, not
the organizer. Guests can't authenticate, so they're the one case the
organizer *does* add directly: `POST /tournaments/{id}/participants`
(guest name + free-text commander, organizer-only).

### Pairing heuristic, not a certified Swiss implementation

`internal/tournaments/pairing.go` is deliberately documented as a pragmatic
heuristic:

- **Table sizes** (`tableSizes`): maximize 4-tables, patch the remainder to
  avoid any table outside [3,4]. Every participant count works except 1, 2,
  and 5 — the Frobenius/Chicken-McNugget gap for coprime 3 and 4 — which
  `POST /tournaments/{id}/start` rejects with a message telling the
  organizer to add or remove one participant.
- **Round count** (`roundCount`): a fixed staircase (N≤8→3, ≤16→4, ≤32→5,
  ≤64→6, else→7), not derived from tournament theory — pods carry more
  information per round than 1v1 Swiss, so fewer rounds converge
  reasonably. A deliberate simplification the user delegated ("automatic
  formula") rather than a researched constant.
- **Pairing** (`pairRound`): greedily seats the next participant (by
  current standings, shuffled for round 1 since everyone starts at 0) at
  whichever open table currently has the fewest repeat opponents, falling
  back to the least-bad table when a repeat is unavoidable. Verified by
  hand and by test (`pairing_internal_test.go`) that it finds a fully
  repeat-free regrouping when one exists (9 players, three 3-tables — a
  3×3 Latin-square-shaped case) and still produces a complete, valid
  pairing when repeats are mathematically unavoidable (8 players, two
  4-tables). Not a provably-optimal Swiss pairer — good enough at the
  scale this feature targets, not tournament-software-grade.

### Standings: plain points, no tiebreakers

`tournament_participants.points` is a running total, incremented by
`POST /tournaments/{id}/tables/{tableId}/result`. Ties are broken only by
registration order for display determinism — no opponents'-win-percentage
or similar Swiss tiebreaker is computed. Consistent with the rest of the
scope: a real tiebreaker system is a distinct, non-trivial feature on top
of this one, not a detail to fold in silently.

## Alternatives considered

- **Wire tournament tables into `games`/`game_players` from the start**:
  rejected for v1, see Decision above (guest support, proxy-join
  authorization). Revisit once there's a concrete need to track a
  tournament table's life totals live.
- **Tie tournaments to an existing `playgroup_id`** (participants drawn
  from its members): simpler authorization story, but incompatible with
  guest participants and with ad-hoc in-person events that don't already
  have a playgroup — rejected in favor of the standalone design, confirmed
  with the user.
- **Organizer-driven registration for app users too** (organizer browses
  and adds any user, picking their deck for them): would require exposing
  another user's deck list to an organizer with no shared playgroup —
  a real authorization gap, not just a UX one. Self-service registration
  sidesteps it entirely: the caller can only ever see and pick their own decks.
- **Let the organizer set the round count manually**: rejected per the
  user's explicit choice of an automatic formula.

## Consequences

- A tournament's "match" is not a real-time, life-tracked game — anyone
  expecting `GET /tournaments/lookup` to drop them into the existing
  WebSocket life tracker will find only a read-only table assignment.
  Documented as the deliberate v1 scope, not a bug.
- Editing or removing a participant, or resizing a table, once a
  tournament has started is not supported — the roster and pairings are
  locked in at `POST /tournaments/{id}/start`.
- No Android UI consumes this API yet; `GET /tournaments/lookup?code=` and
  the rest of the endpoints are shaped so a future screen can, but that's
  unbuilt.
- `internal/tournaments` is fully self-contained: it can be deleted or
  substantially reworked (e.g. to add live-tracker integration) without
  touching `games`/`game_players`/`playgroups` at all.

## References

- `backend/migrations/00016_tournaments.sql`
- `docs/database/schema.dbml` (`tournaments` and related tables)
- `backend/internal/tournaments/` (`pairing.go`, `service.go`, `handler.go`)
- `docs/api/openapi.yaml` (`/tournaments*` paths and schemas)
- `web/app/pages/tournaments/`, `web/app/composables/useTournaments.ts`
- [ADR-0013](0013-proxy-join-y-autorizacion-de-acciones.md) (the proxy-join
  authorization model this ADR's Decision section explains why it doesn't
  cover a tournament organizer)
