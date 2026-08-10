# ADR-0005: Live synchronization protocol over WebSocket

**Status:** Accepted and partially implemented (2026-07-27) — server
implemented (`internal/websocket/`); the Android client is still pending
(see Stage 6 in `docs/roadmap/TASKS.md`).

## Context

The game engine (`internal/games`, `internal/game-actions`) is already
real: `POST /games/:id/actions` records actions (`LifeChange`,
`CombatDamage`, `CommanderDamage`, `PoisonCounter`, `TurnStart`, `TurnEnd`,
`Elimination`) and mutates the real state of the affected player
(`life_total`, `poison_counters`, `is_eliminated`). The problem: if two
players are sitting in the same game, player A's client has no way to
learn about an action taken by player B except by manually polling
`GET /games/:id/timeline` or `GET /games/:id`. For a live life-tracking app
during a Commander game, polling is unacceptable in terms of latency and
cost (N clients asking every X seconds for every active game).

This ADR defines the minimal WebSocket protocol to close that gap: which
events are broadcast, to whom, in what message format, how the connection
is authenticated (the existing auth model is 100% Bearer JWT over HTTP
headers, which doesn't directly apply to a WebSocket handshake from a
browser), and what happens to the connection during the game's lifecycle.

Scope of this pass: **server only** (`internal/websocket/`, wired to
`game-actions`/`games`). The Android client that consumes this protocol
(connection, reconnection with backoff, applying incoming events to the
`GameState`) is the last task of Stage 6 and is not addressed here.

## Decision

### 1. What is broadcast, and to whom

**All seven `game_actions` actions are broadcast without exception**
(`LifeChange`, `CombatDamage`, `CommanderDamage`, `PoisonCounter`,
`TurnStart`, `TurnEnd`, `Elimination`), uniformly, plus one lifecycle event
(`game_finished`) when the game ends.

No subset of "more important" actions is filtered out or chosen, because:

- All seven already share a single shape in the REST API
  (`GameActionResponse`: `action_type` + free-form `payload`); the client
  already knows how to interpret all of them to render the timeline.
  Reusing exactly the same DTO live avoids maintaining two event
  vocabularies.
- If an eighth `action_type` is added tomorrow to the `game-actions`
  vocabulary (`isValidActionType`), it's automatically broadcast without
  touching `internal/websocket`.

The recipient of each event is **every connection subscribed to that
`game_id`** — it is not filtered by whether the connected user is a player
in that game (see "Out of scope" below, authorization point).

`games.JoinGame` / `LeaveGame` / `StartGame` (game transitions in the
`pending` state) **are not broadcast** in this pass: they happen before
there's anything to track live (the game hasn't even started), and the
current flow is "everyone sits down, someone taps start" on the same
screen — there's no demonstrated need to see it live yet. It remains a
natural extension if that need arises.

### 2. Message envelope format

Every message the server sends over the socket uses the same JSON envelope:

```json
{
  "type": "game_action",
  "game_id": "6e59b99a-...-uuid",
  "actor_id": "b4c9d1d0-...-uuid",
  "payload": { "...": "..." },
  "timestamp": "2026-07-27T14:32:01Z"
}
```

- `type`: one of `connected`, `game_action`, `game_finished`, `error` (see
  below).
- `game_id`: always present, redundant with the room the connection is
  subscribed to (simplifies the client: it doesn't need to remember which
  game each socket belongs to if it can already read it from the message).
- `actor_id`: who originated the event; empty/omitted on events with no
  natural actor (`connected`, `game_finished`, `error`).
- `payload`: specific to `type`; see per-event detail below.
- `timestamp`: server time at the moment the message is emitted (RFC3339,
  UTC), **not necessarily equal** to `GameActionResponse.created_at`
  (which is the persistence time in Postgres) — these are slightly
  different events (an action is persisted, and then, separately,
  broadcast), although in practice they happen within the same request and
  differ by microseconds.

By type:

- **`connected`**: sent by the server exactly once, right after the
  connection authenticates successfully. Empty `payload`. Serves as an
  ack: the client knows it's already subscribed and can stop showing a
  "connecting" spinner.
- **`game_action`**: `payload` is exactly a `GameActionResponse` (the same
  DTO already returned by `POST /games/:id/actions` and
  `GET /games/:id/timeline` — `id`, `game_id`, `actor_id`, `target_id`,
  `action_type`, `payload`, `created_at`). The envelope's `actor_id` is
  the same as `payload.actor_id`, duplicated at the envelope level so the
  client can filter/route without deserializing the full payload.
- **`game_finished`**: empty `payload`. It's a notice, not a snapshot — the
  client must request the real final state via REST (`GET /games/:id`,
  `/statistics/*` endpoints) if it needs it, instead of the server
  duplicating that information over two channels. See "REST remains the
  source of truth" below.
- **`error`**: only used during the authentication handshake (see section
  3), never after the connection has been authenticated.
  `payload: { "message": "..." }`.

**REST remains the source of truth.** The WebSocket is a *notification*
channel that something changed (and, for `game_action`, exactly what
changed), not an alternative source of truth nor a mechanism with delivery
guarantees. Faced with any synchronization doubt (reconnection, lost
message, race condition on connect), the client reconciles against
`GET /games/:id` / `GET /games/:id/timeline`. This decision is what allows
explicitly leaving message replay out of scope (section 5): the cost of
not having it is "one extra round trip to REST in the worst case", not
state loss.

### 3. Connection authentication

**Decision: an initial auth message after connecting**, not a JWT via
query param or subprotocol.

The client opens the WebSocket without credentials in the HTTP handshake,
and as the **first text message** (with a 10s timeout) must send:

```json
{ "type": "auth", "token": "<access token JWT, the same one from Authorization: Bearer>" }
```

The server validates the JWT with the same logic already used by
`auth.RequireAuth` (`auth.VerifyAccessToken`, a new exported function that
wraps the signature/expiration verification already existing in
`internal/auth/token.go` — the verification logic is not duplicated). If
valid, it responds `connected` and the connection is subscribed to the room
for the `game_id` from the URL. If the message doesn't arrive in time, isn't
valid JSON, isn't `type: "auth"`, or the token is invalid/expired, the
server sends an `error` with the reason and closes the socket with code
`1008` (Policy Violation).

The other two standard ways of solving this were discarded:

- **JWT as a query param in the handshake URL**
  (`GET /ws/games/:id?token=...`): the simplest option to implement and
  the most common in tutorials, but discarded for two concrete reasons
  specific to *this* backend, not generic ones:
  1. `main.go` already has Fiber's `logger.New()` as global middleware,
     which logs the full path of every request — including the query
     string — for **every** HTTP request, and the WebSocket handshake is a
     normal HTTP request before the upgrade. An access token in the URL
     would end up in the server's plaintext logs on every connection,
     something already deliberately avoided for the rest of the API
     (Bearer token goes in a header, not the URL, precisely so it doesn't
     appear in access logs or intermediate proxy histories).
  2. It would be necessary to decide what to do with that query param in
     any future reverse proxy or CDN in front of the backend (several
     cache or log querystrings by default) — a problem the initial auth
     message doesn't have, because the token never travels in the URL.
- **JWT as `Sec-WebSocket-Protocol` (subprotocol)**: avoids the URL
  logging problem, but is a semantically incorrect use of the field (it's
  meant for negotiating which application protocol is spoken over the
  socket, not for transporting credentials) and has character/length
  restrictions that force encoding the JWT in non-standard ways on some
  clients. The initial auth message achieves the same result (zero tokens
  in the handshake URL/headers) without fighting those restrictions, at
  the cost of a small intermediate state ("connected but not yet
  authenticated") that must be handled with a timeout — a cost considered
  acceptable and already resolved in the implementation.

This is consistent with the rest of the project's auth model
([ADR-0001](0001-auth-jwt-refresh-token-strategy.md)): it remains
Bearer-JWT-only, with no new cookies or server-side sessions; the WebSocket
only changes *how* the same token travels, not what token it is or how
it's issued.

### 4. Connection lifecycle

- **Connect**: `GET /api/v1/ws/games/{game_id}` (public route, without
  `auth.RequireAuth` — authentication happens via the initial message, not
  via the upgrade request's header). It's validated that `{game_id}` has
  UUID format before accepting the upgrade (400 if not); it is **not**
  validated that the game exists or that the authenticated user is a
  player in it (see "Out of scope").
- **Authenticate**: see section 3. Success → `connected` + subscribed.
  Failure → `error` + `1008` close.
- **While the game lasts (`active`)**: every `game_action` successfully
  recorded by `POST /games/:id/actions` is broadcast to the room.
  Best-effort: if a connection has its outgoing buffer full (slow or stuck
  client), that particular message is dropped *only for that connection*
  — it never blocks the HTTP request that originated the action nor
  affects the room's other connections.
- **A client disconnects** (closes the app, loses network, refreshes the
  page): the server detects it when the next read/write on that socket
  fails, and removes it from the room. No one else needs to be notified —
  there are no presence events in this pass (see "Out of scope").
- **The game finishes** (`FinishGame`, via `games.Broadcaster`):
  `game_finished` is broadcast to the whole room and the server **actively
  closes all connections** in that room (code `1000`, normal closure).
  Justification: once `finished`, `game-actions.RecordAction` rejects any
  new action (`game is not active`, 409) — there can never be another
  `game_action` for that room again, so keeping the socket open would only
  consume a file descriptor with no purpose. A client that wants the final
  result already knows to request it via REST (`GET /games/:id`,
  `/statistics/*` endpoints).
- **Reconnection**: there's no session continuity between connections — a
  reconnection is indistinguishable from a new connection (new `auth`
  message, new entry in the room). The client is responsible for, upon
  (re)connecting, refreshing its state from REST before or in parallel with
  subscribing (avoids missing actions that occurred during the outage) and
  for deduplicating by `GameActionResponse.id` if an event it already
  applied via REST also arrives later via WebSocket (this actually always
  happens for the author of the action itself: they receive their
  `GameActionResponse` as the `POST` response, and an identical copy later
  via WebSocket — this isn't a bug, it's the consequence of broadcasting to
  "the whole room" without excluding the sender, see "Out of scope").

### 5. Out of scope for this pass

Explicitly documented so "not implemented" isn't confused with "forgotten":

- **Delivery guarantees / replay on reconnect**: there's no queue of
  pending messages or "what you missed" buffer. If a connection isn't
  subscribed at the moment of a `Broadcast` (hasn't connected yet, dropped,
  or its buffer was full), that message is lost for it permanently.
  Mitigated by the point in section 2 (REST is the source of truth; the
  client reconciles). Justification for deferring it: implementing it
  properly requires deciding where that buffer lives (in process memory?
  is it lost anyway if the process restarts? for how long? a persistent
  log like Kafka/Redis Streams?) — a non-trivial infrastructure decision
  that isn't justified without real data on how often a network outage
  happens during a game.
- **Player-level authorization**: any authenticated user (valid JWT) can
  subscribe to **any** `game_id`, whether it exists or not, whether they're
  a player in that game or not — the server doesn't validate membership.
  The only control is "having a valid JWT" (same as the rest of the API
  requiring being logged in, but not requiring ownership of the resource
  on several read paths). The real risk is low (`game_id`s are
  non-guessable v4 UUIDs, and seeing another game's room doesn't expose
  more than what `GET /games/:id/timeline` already exposes, an endpoint
  that also doesn't validate membership today), but it's a real gap that's
  documented rather than assumed not to exist.
- **Multi-process scaling / external pub-sub**: the `Hub` lives in the
  memory of a single process (`map[game_id][]conn` protected by a
  `sync.RWMutex`). If the backend runs on more than one replica, two
  players in the same game connected to different replicas **won't see
  each other** — each process only knows about its own connections.
  Solving this requires a shared message bus (Redis Pub/Sub, NATS,
  Postgres `LISTEN`/`NOTIFY`) that there's no need for today: the backend
  runs as a single process (see `docker-compose.yml`, with no horizontal
  scaling/orchestration component yet).
- **Presence** ("which players are connected right now") and **activity
  indicators** (typing/"so-and-so is thinking about their turn"): there's
  no event of this kind. It's a real UX improvement but independent of the
  problem this ADR solves (synchronizing game state), and adding it
  entails its own decisions (what counts as "present": the socket being
  open, or some recent interaction? is the socket's join/leave broadcast,
  even though it doesn't correspond 1:1 to being seated in the game?).
- **Client→server channel over the socket**: the WebSocket is unidirectional
  server→client after the initial `auth` message — the server ignores any
  subsequent message a client sends it. Recording actions remains
  exclusively via `POST /games/:id/actions` (REST). There's no relevant
  latency advantage to moving that POST to the socket for this app, and
  doing so would force duplicating all of
  `game-actions.RecordAction`'s validation/authorization in the socket
  handler.
- **Heartbeat / application-level ping-pong**: ~~no explicit ping/pong
  ticker is implemented~~ **implemented 2026-08-10** (`internal/websocket/client.go`):
  `writePump` sends a WebSocket ping frame every `pingPeriod` (54s);
  `readLoop` sets a `pongWait` (60s) read deadline, refreshed by the
  standard pong (an explicit pong, or any other frame the client's read
  pump processes). If no pong arrives within `pongWait`, `ReadMessage`
  returns a deadline-exceeded error and the connection is torn down like
  any other read failure — closing the room entry, freeing the `Hub`
  memory, and unblocking `writePump` in turn. This replaces the previous
  "depends on the next read/write failing" behavior for the specific
  failure mode this bullet used to flag: a client that goes silent without
  a clean TCP close (e.g. a phone losing signal) is now reaped within
  `pongWait`, not left running forever. Fixing this also surfaced and fixed
  an unrelated pre-existing race: `writePump` used to run in a detached
  goroutine, so gofiber's `*Conn` could be recycled back to its
  `sync.Pool` (as soon as the connection handler function returned) while
  `writePump` was still using it; `Client.startWritePump`/`wait` now make
  the handler block until `writePump` has actually exited before
  returning. Covered by `internal/websocket/client_internal_test.go`
  (real WebSocket connections over a loopback listener, run with
  `-race`), not just reasoned about.

## Alternatives considered (general architecture)

- **Server-Sent Events (SSE) instead of WebSocket**: would be sufficient
  for the current use case (server→client only, see the "client→server
  channel" point above) and simpler to implement over plain HTTP/1.1. It
  was discarded anyway because (a) the roadmap already names this stage
  "Synchronization (WebSocket)" explicitly, and (b) SSE has worse native
  support on Android/OkHttp than WebSocket, which is a first-class citizen
  in the already-chosen Android stack — not worth fighting an SSE polyfill
  in the main client to save the (small) extra complexity of handling the
  WebSocket handshake on the server.
- **Short polling as a "good enough" solution**: it's what exists today de
  facto (nothing, really — there isn't even polling implemented on the
  client). Discarded as a *permanent* alternative precisely because it's
  the problem this ADR solves, not a competing option.

## Consequences

- `internal/auth` gains a new exported function (`auth.VerifyAccessToken`)
  that exposes the JWT verification previously only used internally by the
  `RequireAuth` middleware. Additional attack surface: none new (same
  verification, one more caller).
- `games.Service` and `gameactions.Service` gain a new dependency
  (`Broadcaster`, one method each) injected via constructor, following the
  same pattern as `games.StatisticsRecalculator` — neither imports
  `internal/websocket` directly, avoiding tight coupling and allowing the
  broadcast to be mocked in tests.
- New public route (without `auth.RequireAuth`) on the HTTP surface:
  `GET /api/v1/ws/games/:id`. Documenting it in `openapi.yaml` remains a
  separate pending task (OpenAPI 3.1 doesn't model WebSockets well;
  probably warrants just a note in the equivalent REST path's description
  instead of an attempt at a formal spec).
- The in-memory `Hub` is a known and documented scaling limit (see "Out of
  scope"): moving the backend to multiple replicas requires revisiting
  this ADR first.

## References

- Implementation: `backend/internal/websocket/` (`hub.go`, `client.go`,
  `handler.go`, `envelope.go`, `broadcaster.go`)
- Wiring: `backend/cmd/api/main.go` (`registerModules`)
- Decoupling interfaces: `backend/internal/games/service.go`
  (`Broadcaster`), `backend/internal/game-actions/service.go`
  (`Broadcaster`) — same pattern as `games.StatisticsRecalculator`
- Reused JWT verification: `backend/internal/auth/token.go`
  (`VerifyAccessToken`)
- See also [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (the
  Bearer-JWT auth model this ADR reuses without modification)
