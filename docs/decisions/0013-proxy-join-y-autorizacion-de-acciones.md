# ADR-0013: Proxy-join and action authorization via `game_players.added_by`

**Status:** Accepted (2026-07-28)

## Context

The backend's game model assumes that each `GamePlayer` is created from
the JWT session of whoever joins (`POST /games/{id}/join` always takes the
`user_id` from the token, never from the body). That works for a multi-device
web client, but the Android client is pass-and-play on a single
device (see the design comment in
`android/.../data/repository/GameRepository.kt`): a single
authenticated user per game, and the rest of the local seats never had their
own `GamePlayer` or statistics.

A way was requested for that single device to be able to log an entire
game for a real playgroup (`playgroups`) — so that **all**
the seats assigned to group members generate real statistics, not
just the session owner. That requires an authenticated user to be able to, on
behalf of another, (a) join a game and (b) record their
actions (life, commander damage, etc.).

While investigating point (b), a pre-existing authorization gap was found that is
independent of this request: **`POST /games/{id}/actions`
(`internal/game-actions/handler.go: CreateAction`) never reads the `user_id`
from the JWT** — the body's `actor_id` is accepted as-is, without checking that
it belongs to the caller. Today, any authenticated user who knows a
`game_id` and a `GamePlayer.id` (both visible via `GET /games/{id}`) can
record actions on their behalf. This change closes that gap as a
necessary part of touching the same function needed to add the proxy
permission.

## Decision

### New column: `game_players.added_by`

`uuid null references users(id)` (migration
`00012_game_player_proxy_join.sql`). `null` if the player joined with their
own session (the same behavior as always). If not null, it's the `user_id`
of whoever joined them as a proxy — and that user is then authorized to act
on their behalf.

### Proxy-join: `POST /games/{id}/join` with optional `user_id`

`JoinGameRequest` gains an optional `user_id`. If it's present and matches
the caller, behavior is identical to today (`added_by` stays null). If it's present and
is **different** from the caller (`internal/games/service.go: JoinGame`), the
following is required:

1. The game has a `playgroup_id` (it isn't a Casual game).
2. The caller is a member of that `playgroup_id`.
3. The target `user_id` is also a member of that same `playgroup_id`.
4. The `deck_id` in the body belongs to the **target**, not the caller
   (`resolveOwnedDeckID` already validated this against a `userID` — only which
   one is passed changes).

If any of this fails, the same criterion as the rest of the module applies
(generic `ErrPlaygroupNotFound`/`ErrDeckNotFound`, without distinguishing "does not
exist" from "you don't have permission" — this avoids revealing other users'
memberships or decks).

It is anchored to `playgroup_id` (not "any pair of users who know each
other") on purpose: using the same field that already exists in `games` keeps
the authorization surface scoped to real groups, without inventing a
new trust relationship.

### Discovering the target's decks: `GET /playgroups/{id}/members/{userId}/decks`

Without this there is no way to choose the proxy-join's `deck_id`. Same
authorization criterion as the proxy-join (caller and target both members
of the same group). It lives under `/playgroups` (not under `/decks`)
because the authorization depends entirely on shared membership, not on a
direct relationship between the two users.

### Proxy-record: authorization in `POST /games/{id}/actions`

`game-actions/handler.go: CreateAction` now reads `userID` from
`c.Locals` and passes it to `RecordAction`. `resolveActionSubject` validates,
after resolving the `actor` (`GamePlayer`):

```
authorized := actor.UserID == callerID || actor.AddedBy == callerID
```

If not, `403` (`ErrNotAuthorizedForActor`, new). This is strictly stricter than
current behavior (which validates nothing), so it doesn't break any
existing legitimate flow — every `actor_id` sent today always
corresponds to the caller themselves in practice (the Android client
never sent someone else's).

## Alternatives considered

- **Delegating everything at the game level** (if the caller has *any*
  `GamePlayer` in that game, they can act on behalf of any other seat in
  the same game): simpler to implement, but any player at the table could
  alter any other player's statistics simply by being seated — too
  permissive for something that persists real statistics.
  `added_by` scopes the authority to "whoever actually joined them," which is
  whoever is holding the device.
- **Persistent trust relationship between users** ("delegates"), instead
  of deriving it from `added_by` per game: more flexible (would survive
  across individual games), but is a new table and a new invite/accept flow
  for a use case — logging your own group's table — that already
  has a natural and sufficient trust signal (`playgroup_members`).
  Ruled out until something more granular is needed.
- **Not closing the `POST /games/{id}/actions` gap in this pass**
  (leaving it for a separate ticket): ruled out because the function that needs
  to be touched to add the proxy permission is exactly the one that today doesn't
  validate anything — fixing it now is strictly cheaper than doing it in
  two passes, and knowingly leaving it open would no longer be an oversight but
  a conscious decision to ship with a known vulnerability.

## Consequences

- `game_players.added_by` is the single source of truth for "who can
  act on whose behalf" — there is no explicit revocation (if the game ends or
  the proxy-joiner leaves the group, the authorization still applies to that
  specific, already-finished game, where it no longer matters).
- The Android client (Group mode) is the first and only real caller of
  proxy-join until a multi-device client exists (Stage 6); the
  web client doesn't use it yet.
- Any future extension of "who can see/act on behalf of whom" (e.g.
  roles within a group) should revisit whether `playgroup_members`
  still suffices or a richer permission model is needed — not resolved
  here, only the minimum for this use case.

## References

- `backend/migrations/00012_game_player_proxy_join.sql`
- `backend/internal/games/service.go` (`JoinGame`)
- `backend/internal/game-actions/service.go` (`RecordAction`,
  `resolveActionSubject`)
- `backend/internal/playgroups/service.go` (shared membership)
- `android/app/src/main/java/com/commandercompanion/data/repository/GameRepository.kt`
  (design comment about the single-device pass-and-play model)
- [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (JWT, basis for
  `common.UserIDKey`)
