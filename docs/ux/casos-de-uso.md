# Detailed use cases

Step-by-step user flows for the five core operations of the product: create
a game, join a game, track life during the game, finish a game, and view
statistics.

**How to read this document:** for each use case, two columns are
described — **"Today"** (what the code does right now, verified by reading
`backend/internal/games/service.go`, `backend/internal/game-actions/service.go`
and the Android screens in `presentation/screens/`) and **"Target"** (the
full end-to-end, multi-device flow that the ROADMAP envisions). **Updated
2026-07-27**: Android is no longer 100% local — it authenticates for real
(`LoginViewModel` against `POST /auth/login`/`/auth/google`) and, from
`GameRepository.bootstrapRemoteGame()`, mirrors **best-effort** the local
seat (seat 1, the only one with server-side identity) against the real
backend: `POST /games` + `POST /games/{id}/join` + an attempt at
`POST /games/{id}/start` when creating the game, `POST /games/{id}/actions`
on every life change for that seat (`mirrorLifeChange`), and
`POST /games/{id}/finish` when finishing. It is "best-effort and additive"
(comment from `GameRepository.kt`): if it fails (no network, no session, no
decks) or if the game never reaches 2 players joined from the backend, the
local tracker keeps being played all the same — the reason is reflected in
the `GameTrackerScreen` banner (`RemoteSyncBanner`, `GameState.remoteSync`:
`Disabled`/`WaitingForPlayers`/`Synced`/`Failed`), it doesn't block anything.
Commander damage (other seats) and everything else remains purely local —
see the detail of what is mirrored and what isn't in each use case below.
See `docs/roadmap/TASKS.md`, Stage 4 and 5.

For what each web screen actually looks like, see
[`screenshots.md`](screenshots.md) — real screenshots, not mockups.

---

## 1. Create a game

**Actor:** any user (today: anyone with the Android device in hand; target:
authenticated user).

### Today (Android, local pass-and-play + best-effort mirror of seat 1)

1. User opens the app → `LoginRoute` → authenticates for real: "SIGN IN"
   calls `LoginViewModel` → `POST /auth/login` (email/password), or
   "Continue with Google" → Credential Manager obtains a real `id_token` →
   `POST /auth/google`. Tokens are saved in `SessionManager` (DataStore).
   Only then does it navigate to `DashboardRoute` (with
   `popUpTo(LoginRoute) { inclusive = true }`). `DashboardScreen` also has a
   "Sign out" button (`DashboardViewModel.logout()`, revokes the refresh
   token best-effort).
2. In `DashboardScreen`, taps **"NEW GAME"** → navigates to
   `PlayerSetupRoute`.
3. In `PlayerSetupScreen` (`PlayerSetupScreen.kt`):
   - Chooses the number of players with `FilterChip` (2 to 6, constants
     `MIN_PLAYERS`/`MAX_PLAYERS`).
   - For each player: free-text name (`OutlinedTextField`, default
     `"Player N"`) and a color from the WUBRG + colorless palette
     (`PlayerColorPalette`, circular swatches).
   - Taps **"START GAME"**: a local `gameId` is generated with
     `UUID.randomUUID()` (it is not a backend ID, no `games` row exists in
     Postgres) and the `PlayerConfig` values (name + color) are encoded into
     a string (`encodePlayerConfigs`) that travels as a route argument.
4. Navigates to `PreGameRoute(gameId, playersEncoded)`.
5. In `PreGameScreen` (`PreGameScreen.kt`):
   - **Turn draw**: the "DRAW" button picks a random player
     (`Random.nextInt(configs.size)`) and highlights them with their color;
     the result (`startingSeat`) is propagated to the game as a "· starts"
     badge.
   - **Mulligans**: a ± counter per player (minimum 0, no upper cap) that is
     saved in each `PlayerConfig.mulligans` before starting.
   - Taps **"START GAME"** again → navigates to `GameTrackerRoute`, doing
     `popUpTo(DashboardRoute)` (setup and pre-game leave the back stack).
6. On entering `GameTrackerScreen`, `GameViewModel.init` triggers two things
   in parallel: `persistNewGame()` inserts into Room (Hilt
   `DatabaseModule`, `GameDao`) a `GameEntity` with `status = "IN_PROGRESS"`
   and a `PlayerResultEntity` per player (starting life 40, color,
   mulligans) — this is always local and unconditional; and
   `bootstrapRemoteGame()` (best-effort, see note at the top of this
   document) calls `POST /games` and seats seat 1 (the authenticated user)
   with their first deck via `POST /games/{id}/join`, also attempting
   `POST /games/{id}/start` (which stays `pending` with `409` if no one else
   joined from the backend — this is not an error, see
   `GameRepository.bootstrapRemoteGame`). If the user has no decks or the
   call fails, the game keeps being played 100% local with no other
   consequence than the status banner.

### Target (real backend, Stage 5)

1. An authenticated user (JWT Bearer) calls `POST /games` with an optional
   `playgroup_id` (`games/service.go: CreateGame`). A row is created in
   `games` with `status = "pending"`, with no players yet.
2. Each player (including the creator) joins explicitly via
   `POST /games/{id}/join` (see use case 2) — creating the game and
   "sitting down" in it are two separate steps in the backend, unlike today
   in Android where configuring players and creating the game is a single
   step.
3. When there are ≥ 2 players joined, someone calls `POST /games/{id}/start`,
   which transitions `pending → active` (see
   `docs/diagrams/game-state-machine.md`).
4. In Android, this would replace step 3-4 of "Today": instead of
   generating a local `gameId` and encoding players into the route,
   `PlayerSetupScreen` would call `POST /games` + N × `POST /games/{id}/join`
   (with the `deck_id` of each authenticated player) before navigating to
   `PreGameScreen`.

**Key divergence:** the backend models "create" and "join" as separate steps
of a multi-user game with authentication and real decks; today Android
collapses them into a single local configuration flow with no users or
decks, because all players share the same physical device.

---

## 2. Join a game

**Actor:** authenticated user with at least one deck of their own.

### Today (Android)

It does not exist as an independent use case. "Joining" in practice is
adding one more row to the player list in `PlayerSetupScreen` (raising the
count `FilterChip` from 2 to 6) — there is no concept of inviting,
accepting, or per-player user identity. All "players" are simply names +
colors typed in by whoever is holding the phone.

### Target (real backend, `games/service.go: JoinGame`)

1. An authenticated user calls `POST /games/{id}/join` with body
   `{ deck_id }` (the `user_id` does **not** go in the body — it is always
   taken from the JWT, so that no one can enter players under another
   user's name).
2. Server-side validations, in order:
   - The game must exist and be in `status = "pending"` — if not, `409
     "game is not accepting new players"`.
   - The `deck_id` must exist and belong to the authenticated user — if it
     doesn't exist **or** belongs to another user, `404 "deck not found"`
     in both cases (no distinction is made, so as not to reveal other
     users' decks by ID).
   - The user must not already be seated in that game — if they already
     are, `409 "already joined this game"`.
3. If everything is valid, a row is created in `game_players` with starting
   life (`life_total` default from the schema), and joining/playing can
   continue while the game stays `pending`.
4. A player can change their mind and call `POST /games/{id}/leave` while
   the game stays `pending` (`LeaveGame`); once `active`, `409 "cannot leave
   a game that already started"`.

**Key divergence:** in the backend, "joining" implies authentication +
ownership of a specific deck; in Android today there is neither a session
nor a deck associated with each player in the local game.

---

## 3. Track life during the game

**Actor:** any player (today: whoever is touching the shared device's
screen; target: each player from their own device, synchronized via
backend/Websocket).

### Today (Android, `GameTrackerScreen` + `GameViewModel` + `PlayerCard`)

1. `GameTrackerScreen` builds a dynamic grid: players are grouped 2 per row
   (`state.players.chunked(2)`), it works the same for 2 as for 6 players
   (there is no longer a layout fixed to 4).
2. **Turn counter** in the header: `<`/`>` buttons increment or decrement
   `state.currentTurn` (`GameViewModel.nextTurn/previousTurn`). It is purely
   local to the session, is not persisted as an event, and does not
   indicate whose turn it is (just a single global number).
3. **Life** (`PlayerCard`): each card shows the player's name (with a "·
   starts" suffix if `isStartingPlayer`), the mulligan badge if
   `mulligans > 0`, and the large life number in the center with `-`/`+`
   buttons that call `onLifeChange(-1)`/`onLifeChange(1)` →
   `GameViewModel.adjustLife(playerId, amount)`. Each adjustment triggers
   `checkForGameOver()`.
4. **Commander damage**: tapping the whole card (`clickable` on the
   `Surface`) toggles an overlay (`showCommanderDamage`) that shows, in a
   grid of up to 3 columns, one item per **other** player (`otherPlayers`)
   with their color, the accumulated commander damage received from that
   attacker, and `-`/`+` buttons. Raising an attacker's damage also
   subtracts that same amount from the player's total life (`life = life -
   amount` in `adjustCommanderDamage`) — that is, "commander damage" is not
   a counter separate from life, but a per-opponent-memory way of
   subtracting life.
   - **Known limitation**: the model (`PlayerState.commanderDamage: Map<Int,
     Int>`) tracks accumulated damage per opponent but does not implement
     the rule that "21 commander damage from a **single source** eliminates"
     — it simply subtracts life; the player is only eliminated when their
     total life reaches 0 (same limit as in the backend, see below).
5. Every life change or commander damage change calls `checkForGameOver()`:
   if exactly 1 player is left with `life > 0` (and there is more than 1
   player in the game), that game is automatically finished with that
   player as the winner (see use case 4).
6. Everything lives in `_state: MutableState<GameState>` in memory until the
   game is finished (see use case 4); if the process dies before finishing,
   the in-progress life state is lost (documented as out of scope in
   `TASKS.md`). **Real exception**: every life change for **seat 1** (the
   authenticated user) does trigger a request — `adjustLife` calls
   `mirrorLifeChange(amount)` → `POST /games/{id}/actions`
   (`action_type: LifeChange`, no `target_id`, see
   `GameRepository.recordLifeChange`) if the remote session is `active`; if
   it fails or there is no active session, it is a silent no-op except for
   updating the status banner. The other seats (2-6) and commander damage
   from **any** seat are never mirrored (see the comment on
   `adjustCommanderDamage` in `GameViewModel.kt`: attributing that damage to
   the local seat as `actor_id` would credit someone else's
   `total_commander_damage_dealt`).

### Target (real backend, `game-actions/service.go`)

1. Every life/damage change would be recorded as `POST /games/{id}/actions`
   with `{ actor_id, target_id?, action_type, payload: { amount } }`, where
   `actor_id`/`target_id` are `game_players` IDs from **that** game (not
   `user_id`), and `action_type` is one of: `LifeChange`, `CombatDamage`,
   `CommanderDamage`, `PoisonCounter`, `TurnStart`, `TurnEnd`,
   `Elimination`.
2. The backend validates that the game is `active` (`409` if not) and that
   actor/target belong to that game (`404` if not), and **mutates the real
   state** of the affected player, not just logs the event:
   - `LifeChange` adjusts `life_total` in either sign.
   - `CombatDamage`/`CommanderDamage` subtract from `life_total` (same
     effect as today — the schema does not distinguish the origin of
     commander damage per player, so **the backend does not yet implement
     either** the rule of 21 from a single source; see `applyAction` in
     `game-actions/service.go`, explicitly commented as a known
     limitation).
   - `PoisonCounter` adjusts `poison_counters`.
   - `TurnStart`/`TurnEnd` remain log-only markers (the `games` schema has
     no column yet for "whose turn it currently is").
   - **Server-side auto-elimination**: if after an adjustment
     `life_total <= 0` or `poison_counters >= 10`, the player is
     automatically marked `is_eliminated = true` (standard Commander rules)
     — equivalent to what Android decides locally today by checking
     `life > 0`.
3. `GET /games/{id}/actions` exposes the full timeline, ordered
   chronologically, to reconstruct the game (also used by
   `statistics.RecalculateForGame`, see use case 5).
4. Rebroadcasting these events in real time to every device seated in the
   game is Stage 6 (Websocket): the server (`internal/websocket/`) already
   rebroadcasts the 7 actions via `GET /api/v1/ws/games/{id}` (see
   [ADR-0005](../decisions/0005-websocket-protocol.md)); what's missing is
   the Android client that connects to that socket — today the local seat's
   mirror (above) is REST-only, one-way, with no subscription to other
   players' changes.

**Key divergence:** today life lives only in the memory of the device that
is touching it, with no record of individual actions or of who caused each
change; the backend already has a full auditable action engine per player,
but no Android screen calls it yet.

---

## 4. Finish a game

**Actor:** any player present (today); the backend does not distinguish who
can finish (any authenticated request to that endpoint can do it, there is
no "host" role).

### Today (Android, `GameViewModel.finishGame`)

1. Two possible triggers:
   - **Automatic**: `checkForGameOver()` detects that only 1 player is left
     with `life > 0` after any life/damage change, and calls
     `finishGame(winnerId = alive.first().id)` directly.
   - **Manual**: the player taps **"Finish"** in the header →
     `showFinishConfirm = true` → confirmation `AlertDialog` ("Each
     player's current life will be recorded in the history") → confirming
     calls `finishGame()` with no explicit `winnerId`.
2. If finished manually without a clear winner, `finishGame` resolves the
   winner as the player with the most life among those with `life > 0`,
   **only if unique** (`count { it.life == player.life } == 1`); if there
   is a tie at the maximum (2+ players with the same highest life) or
   everyone is at 0 or below, `winnerId = null` → the game is left "without
   a winner".
3. `isFinished = true` is set and `persistGameResult` is triggered: in
   Room, `gameDao.finishGame(gameId, status = "FINISHED", endTime = now)`
   and, for each player, `updatePlayerResult(gameId, seatIndex, finalLife,
   won = (id == winnerId))`.
4. A final `AlertDialog` is shown: title "{winner} wins!" or "Game finished"
   if there is no winner, and the final life detail for each player. "Back
   to home" does `onFinish()` →
   `navController.popBackStack(DashboardRoute, inclusive = false)`. In
   parallel, `finishRemoteGame()` calls `POST /games/{id}/finish` on the
   remote session if it was `active` (best-effort, same criterion as the
   rest of the mirror) — this is what triggers the actual statistics
   recalculation on the backend side for seat 1, even though the Android UI
   doesn't show that result yet (see use case 5).
5. Once `isFinished = true`, both `adjustLife` and `adjustCommanderDamage`
   and `finishGame` go back to being no-ops (`if
   (_state.value.isFinished) return` is checked at the start) — the game
   stays frozen.

### Target (real backend, `games/service.go: FinishGame`)

1. `POST /games/{id}/finish` is only valid if the game is `active` — `409
   "only an active game can be finished"` in any other state (including
   trying to finish one that's already finished).
2. Unlike Android, the endpoint **does not receive an explicit winner**: the
   game transitions to `status = "finished"` with `finished_at` set, and
   who won is **derived afterward**, in `statistics.RecalculateForGame`
   (triggered automatically within the same logical transaction as
   `FinishGame`, via the `StatisticsRecalculator` interface), using the
   same "single non-eliminated survivor" criterion (`is_eliminated =
   false`) — if there are 2+ survivors because the game was cut short
   manually before reaching 1, no one is credited with a win, although
   `games_played` is still counted for all participants.
3. This recalculation also updates `user_statistics_summary` and
   `deck_statistics_summary` (damage dealt, highest life reached, etc.),
   which is the basis for use case 5.

**Key divergence:** Android decides the winner **on the client**, at the
moment of finishing, with a somewhat different rule (highest life, with a
tie counted as "no winner"), instead of "single non-eliminated survivor";
the backend decides it **on the server**, after finishing, with the
"single survivor" rule. Both rules agree in the common case (1 player left
with life > 0) but can diverge in games cut short manually with more than
one player still alive.

---

## 5. View statistics

**Actor:** any user (target); in Android today there is no concept of user,
only of device.

### Today (Android, `HistoryScreen` + `HistoryViewModel`)

1. From `DashboardScreen`, the **"HISTORY"** button → `HistoryRoute`.
2. `HistoryViewModel` exposes `gameDao.getGamesWithPlayers()` as
   `StateFlow<List<GameWithPlayers>>` — **it is local game history for this
   device, not aggregated statistics** (no win rate, no damage totals, no
   charts).
3. For each game (`GameHistoryCard`): formatted start date/time, status
   ("Finished" / "In progress"), winner's name if there is one (or the
   number of players if not), and a row with the color + name + final life
   of each player ordered by seat, with a `(Nm)` suffix if they had
   mulligans.
4. If there are no saved games, the message "No games recorded yet" is
   shown.
5. **There is no aggregated statistics screen in Android today** (per user,
   per deck, or per group) — it is raw game-by-game history, and it is also
   tied to local Room: if the app is uninstalled or the device is changed,
   it is lost.

### Target (real backend, `internal/statistics`)

Three endpoints already implemented and with integration tests, but **with
no Android screen consuming them yet** — unlike use cases 1-4, this is not
blocked by a lack of backend connection (Android already talks to
`games`/`game-actions`, see above): it is simply that there is not yet a
screen that calls `CommanderApi.getUserStats`/`getDeckStats`/
`getPlaygroupStats` (all three methods are already in the interface, see
Stage 4 of `TASKS.md`), nor a `StatisticsRepository`. The web client (Nuxt)
already consumes them: `app/pages/statistics.vue` (user/deck) and
`app/pages/playgroups/{index,[id]}.vue` (per group):

- `GET /statistics/user/{id}`: `games_played`, `games_won`,
  `total_damage_dealt`, `total_commander_damage_dealt`,
  `total_eliminations` accumulated from all of the user's finished games.
  If the user never finished a game, it returns zeros (not `404`).
- `GET /statistics/deck/{id}`: the same per deck, plus
  `highest_life_total_achieved` (recalculated by replaying the action log
  from the baseline of 40, not just the final `life_total`). Requires
  ownership of the deck (404 if it does not belong to the authenticated
  user).
- `GET /statistics/playgroup/{id}`: **live** aggregation (there is no
  summary table per group) over that playgroup's finished games: games
  played/won per member, using the same "single survivor" criterion for
  the winner.
- **Known limitation documented in the backend**: `total_eliminations` only
  counts explicit `Elimination` actions with a target different from the
  actor — auto-eliminations from life/poison (the most common way to end a
  Commander game) are not attributed to a specific actor in the log, so
  they don't add to anyone's elimination statistics.

**Key divergence:** "viewing statistics" today in Android is actually
"viewing local game history" (no aggregation, no user, no network); the
backend already has a real aggregated statistics engine per user/deck/
group, and Android already calls it for other purposes (games/game-actions),
but there is still no screen or repository that consumes `/statistics/*` —
pending in Stage 7, no longer blocked by Stage 5 (which in practice has
already been partially resolved via the best-effort mirror).

---

## Summary of the Android ↔ Backend gap

| Use case | Backend (`internal/games`, `internal/game-actions`, `internal/statistics`) | Android (today) |
|---|---|---|
| Create game | `POST /games` → `pending`, multi-user, requires auth | Local (`gameId` = random UUID) **+** best-effort mirror: `POST /games`+`join`+`start` for the seat marked "This is me" |
| Join | `POST /games/{id}/join`, deck ownership, scoped to `pending` | No invite/join UI exists: "joining" in the UI = adding one more player in the local setup; the only real `join` is the automatic one for the local seat (chosen in `PlayerSetupScreen`, with its deck) during bootstrap |
| Track life | `POST /games/{id}/actions`, auditable timeline, server-side auto-elimination | In memory (`GameViewModel`) for all seats; **only the local seat** also mirrors `LifeChange` via `POST /games/{id}/actions` |
| Finish | `POST /games/{id}/finish`, only from `active`, winner derived post-hoc | Winner decided on the client **+** best-effort `POST /games/{id}/finish` for the local seat's remote session |
| View statistics | `GET /statistics/{user,deck,playgroup}/{id}`, real aggregates | `HistoryScreen`: raw Room history, no aggregation; no screen or repository for `/statistics/*` yet (the web client does consume them) |

The gap that remains is not "Android doesn't talk to the backend" (it
already does, best-effort, for the local seat) but rather: (1) live
synchronization of what **other** devices/players in the same game are
doing — this requires the Stage 6 WebSocket client, which consumes a
protocol already implemented on the server side
([ADR-0005](../decisions/0005-websocket-protocol.md)); and (2) a
statistics screen in Android. Deck selection and "which seat am I" (formerly
hardcoded to seat 1 + the user's first deck) is now explicit in
`PlayerSetupScreen` (2026-07-28). See `docs/roadmap/TASKS.md`, Stage 4/5/6,
for the piece-by-piece detail.
