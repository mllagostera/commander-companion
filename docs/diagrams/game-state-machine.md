# Diagram: state machine of a game (backend)

Source of truth for behavior: `backend/internal/games/service.go`
(transitions) and `backend/internal/game-actions/service.go` (what can be
done within each state, and the individual player's auto-transitions on
elimination). All invalid transitions return `409 Conflict`; a
non-existent or non-parseable game `id` returns `404`.

## Game states and transitions

```mermaid
stateDiagram-v2
    [*] --> pending: POST /games\n(CreateGame)

    pending --> pending: POST /games/{id}/join\n(JoinGame — requires own deck,\nuser can't join twice)
    pending --> pending: POST /games/{id}/leave\n(LeaveGame — only if a member)
    pending --> active: POST /games/{id}/start\n(StartGame — guard:\n>= 2 players joined)
    active --> finished: POST /games/{id}/finish\n(FinishGame)

    finished --> [*]

    note right of pending
        join / leave only allowed
        in this state. Any
        attempt in active or finished
        returns 409.
    end note

    note right of active
        start is no longer valid (409
        "game already started or
        finished"). Game-actions are
        recorded here (see below).
    end note

    note right of finished
        finish is no longer valid (409
        "only an active game can be
        finished"). Triggers
        statistics.RecalculateForGame
        synchronously within the
        same FinishGame call.
    end note
```

### Exact guards (`games/service.go`)

| Transition | Guard | Error if it fails |
|---|---|---|
| `pending → pending` (join) | game is `pending`; `deck_id` exists and belongs to the authenticated user (JWT); user is not already seated in this game | `409` if not `pending`; `404 "deck not found"` if the deck doesn't exist or isn't yours (same message for both cases, so as not to reveal which); `409 "already joined this game"` |
| `pending → pending` (leave) | game is `pending`; user is a current member | `409 "cannot leave a game that already started"` if not `pending`; `404 "not a member of this game"` if not a member |
| `pending → active` (start) | game is `pending`; `len(players) >= minPlayersToStart` (constant = 2) | `409 "game already started or finished"` if not `pending`; `409 "not enough players to start"` if there are < 2 players |
| `active → finished` (finish) | game is `active` | `409 "only an active game can be finished"` in any other state |

`minPlayersToStart = 2` is a constant in the code
(`games/service.go:18`), not configurable via environment variable today.

## Sub-state: actions and player elimination within `active`

While the game is `active`, each player (`game_players`) has their own
lifecycle independent of the game's overall state, governed by
`game-actions/service.go`:

```mermaid
stateDiagram-v2
    [*] --> alive: AddGamePlayer\n(on join, starting life_total)

    alive --> alive: LifeChange / CombatDamage /\nCommanderDamage / PoisonCounter\n(POST /games/{id}/actions)
    alive --> eliminated: life_total <= 0\n(auto, after any life adjustment)
    alive --> eliminated: poison_counters >= 10\n(auto, after any poison adjustment)
    alive --> eliminated: action_type = Elimination\n(explicit, target != actor)

    eliminated --> [*]

    note right of eliminated
        is_eliminated = true.
        There is no transition back
        to "alive" — there is no
        "revive" action_type.
    end note
```

**Important:** `RecordAction` is only accepted if the parent game is
`active` (`409 "game is not active"` in any other state) — regardless of
whether the affected player is already eliminated or not; the schema does
not block recording actions on a player who is already eliminated.

Valid `action_type` values (closed vocabulary, `isValidActionType`):
`LifeChange`, `CombatDamage`, `CommanderDamage`, `PoisonCounter`,
`TurnStart`, `TurnEnd`, `Elimination`. Any other value → `400 "invalid
action_type"`.

## How the winner is determined at finish time

`FinishGame` does not receive an explicit winner. The `active → finished`
transition is unconditional (once the state guard has passed); the winner
is calculated **afterward**, in `statistics.RecalculateForGame` (triggered
within the same `FinishGame` call, via the `StatisticsRecalculator`
interface):

- **Single survivor** (`is_eliminated = false`) among the game's
  `game_players` → that player is credited `games_won +1`.
- **2 or more survivors** (game cut short manually before reaching 1 player
  alive) → no one is credited with the win, but all participants still get
  `games_played +1` counted.

## Known limitations (documented in the code itself)

- `CommanderDamage` today behaves the same as `CombatDamage`: it subtracts
  from aggregate `life_total`, without distinguishing the damage source per
  opponent (real Commander rule: 21 commander damage from a single source
  eliminates). The schema (`game_players`) has no table for damage per
  player-commander pair — it would require a new migration.
- `TurnStart`/`TurnEnd` are log-only markers: `games` has no column for
  "whose turn it currently is", so these actions do not mutate any state,
  they just stay in the timeline (`GET /games/{id}/actions`).
