# Commander Companion — Task list

Compact status checklist, organized by the **Stages** defined in [ROADMAP.md](ROADMAP.md). This document is the source of truth for day-to-day progress: the ROADMAP explains the *what* and the *why*, this file tracks the *status*.

The full narrative behind any item — what changed, why, gotchas hit, how it was verified, dates, PRs, exact user requests — lives in **[DECISIONS-LOG.md](DECISIONS-LOG.md)**, organized by the same stages. This file only keeps what's needed to know current status at a glance; read the log when you need the "why" or the history behind a decision.

**How to keep it up to date:**
- Mark `[x]` when a task is finished and functionally complete (not just "it compiles").
- If something is halfway done (scaffolding without real logic, stub, dummy data), leave it as `[ ]` and add a short note in parentheses explaining what's missing.
- Add new tasks as they come up; don't delete completed ones, they're useful history.
- Keep each line short (status + what + a pointer to the file/module). Put narrative, rationale, and verification detail in DECISIONS-LOG.md instead, dated, under the matching stage — don't let it accumulate here again.
- Update the "Last reviewed" date every time the real state of the code is audited, and add the corresponding entry to DECISIONS-LOG.md.

**Last reviewed:** 2026-08-05 — see [DECISIONS-LOG.md](DECISIONS-LOG.md) for the full history of audits and decisions up to that date, including this restructuring itself.

---

## Stage 0: Functional definition

- [x] Roadmap (`docs/roadmap/ROADMAP.md`)
- [x] Architecture and principles document (`docs/architecture/ARCHITECTURE.md`)
- [x] Detailed use cases — [`docs/ux/casos-de-uso.md`](../ux/casos-de-uso.md)
- [x] Android wireframes — [`docs/ux/wireframes.md`](../ux/wireframes.md)
- [x] Flow/state diagrams — [`docs/diagrams/game-state-machine.md`](../diagrams/game-state-machine.md), [`docs/diagrams/android-navigation-flow.md`](../diagrams/android-navigation-flow.md)
- [x] Foundational ADRs (retroactive, inherited decisions): [0006](../decisions/0006-go-fiber-backend.md) Go+Fiber, [0007](../decisions/0007-postgresql.md) PostgreSQL, [0008](../decisions/0008-sqlc-goose.md) sqlc+goose, [0009](../decisions/0009-android-nativo-vs-crossplatform.md) native Android, [0010](../decisions/0010-monolito-modular-vs-microservicios.md) modular monolith
- [x] Session ADRs: [0001](../decisions/0001-auth-jwt-refresh-token-strategy.md) auth strategy, [0002](../decisions/0002-google-sign-in.md) Google Sign-In, [0003](../decisions/0003-cors-permisivo-en-dev.md) CORS in dev, [0004](../decisions/0004-web-client-nuxt.md) web client, [0005](../decisions/0005-websocket-protocol.md) WebSocket protocol

## Stage 1: Backend (project base)

- [x] Go project initialized (`go.mod`, `cmd/api/main.go`, Fiber)
- [x] Modular structure under `internal/` (auth, users, decks, games, game-actions, playgroups, statistics, sync, websocket, common)
- [x] `playgroups`/`games`/`game-actions` slices generated and registered in `cmd/api/main.go`
- [x] `sqlc generate` running for the 6 modules with queries

### Auth — email/password
- [x] bcrypt password hashing (`internal/users/service.go: RegisterUser`)
- [x] Real login/refresh (rotation)/logout/me
- [x] `auth.RequireAuth` Fiber middleware, applied to all protected routes
- [x] Symmetric HS256 JWT + `JWT_SECRET`, configurable access/refresh TTLs — see [ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)

### Auth — Google OAuth (Sign-In)
- [x] ADR: Google as an additional provider, doesn't replace password — [ADR-0002](../decisions/0002-google-sign-in.md)
- [ ] **Create OAuth credentials in Google Cloud Console** (Web + Android Client ID) — external manual step; backend already reads `GOOGLE_CLIENT_ID`, without it `POST /auth/google` responds `501` instead of crashing
- [x] DB schema: `google_id`, nullable `password_hash`, CHECK constraint (migration `00002_auth.sql`)
- [x] `POST /auth/google`: verifies `id_token` (issuer/audience/JWKS via `go-oidc`), issues the same token pair as login
- [x] Account creation/auto-linking by verified email (`users.FindOrCreateGoogleUser`)
- [x] `GOOGLE_CLIENT_ID` documented in `.env.example`
- [x] Specific error handling: 401 invalid token, 400 unverified email, 501 not configured

### Auth — email verification
- [x] ADR: block login until email confirmed, Resend with dashboard templates — [ADR-0012](../decisions/0012-verificacion-email-resend.md)
- [x] Migration `00011_email_verification.sql`: `users.email_verified`, `email_verification_tokens`
- [x] `RegisterUser` issues + persists the token (hashed, TTL 24h), sends the email (best-effort, doesn't roll back registration on send failure)
- [x] `VerifyCredentials` returns `403` if the password is correct but the email isn't confirmed
- [x] `POST /auth/verify-email`, `POST /auth/resend-verification` (never reveals whether the email exists/is verified)
- [x] Linking a Google account marks `email_verified = true`
- [x] `internal/email`: Resend client + console mailer fallback when `RESEND_API_KEY` is empty
- [x] Web: `register.vue`/`login.vue`/`verify-email.vue` flow
- [x] `REQUIRE_EMAIL_VERIFICATION` flag (default `false`, current alpha phase — accounts created already verified)
- [ ] **Exit alpha: turn on `REQUIRE_EMAIL_VERIFICATION=true`** — needs a verified domain on Resend (SPF/DKIM/DMARC) and a real dashboard Template first; external manual step

### Games / game-actions — game engine
- [x] `games` wired to real `Queries`: create/get/list/join/leave/start/finish
- [x] `pending → active → finished` state machine enforced server-side (join/leave only pending, start needs ≥2 players, finish only active)
- [x] `JoinGame`: self-join by default; `user_id` enables proxy-join, gated by shared playgroup membership — see [ADR-0013](../decisions/0013-proxy-join-y-autorizacion-de-acciones.md)
- [x] `GET /games?playgroup_id=`: full group history, membership-gated
- [x] `game-actions` wired: validates `action_type` against the fixed vocabulary, resolves actor/target as `game_players`, only in `active` games
- [x] Actions mutate real player state (life/poison/elimination); server-side auto-elimination at 0 life / 10 poison / 21 commander damage from one source
- [x] Integration tests covering the full state machine and every `action_type`
- [x] `CommanderDamage` tracked per attacker-defender pair (`commander_damage` table, migration `00007`); `RecordAction` runs inside a single transaction
- [x] `TurnStart`/`TurnEnd` set/clear `games.current_turn_player_id` (doesn't model turn order, only "whose turn is it now")
- [x] `openapi.yaml` updated for `/games/*`
- [ ] **`payload.amount` had no range/sign validation** (found in a 2026-08-01 review, see DECISIONS-LOG.md): negative `CombatDamage`/`CommanderDamage` silently healed the target, and a magnitude above `int32` range wrapped into a large negative number instead of erroring. **Fixed same day**: `CombatDamage`/`CommanderDamage` now require a strictly positive amount, and all `payload.amount` values are bounded to whole numbers with magnitude ≤ 1,000,000 (`ErrAmountOutOfRange`/`ErrAmountMustBePositive`, `internal/game-actions/service.go`); regression tests added. `openapi.yaml` updated with the new validation rules.

### Playgroups — game groups
- [x] `playgroups` wired: auto-join creator, membership-scoped listing/detail
- [x] `AddMember` validations (inviter is a member, target exists, not already a member)
- [x] Integration tests
- [x] Proxy-join via `game_players.added_by` + `RecordAction` actor-ownership authorization — see [ADR-0013](../decisions/0013-proxy-join-y-autorizacion-de-acciones.md)
- [x] `GET /users/search?q=`: username partial + exact-email match, no email leak of others, rate-limited
- [x] `PATCH /playgroups/{id}` rename; `GET /playgroups/{id}/members/{userId}/decks`
- [x] `GET /users/username-available?username=`: public, rate-limited, exact case-sensitive match — used by the web/Android registration forms to validate before submitting (2026-08-05)

### Statistics — real recalculation and queries
- [x] `games/service.go: FinishGame` triggers `statistics.RecalculateForGame(gameID)`
- [x] `RecalculateForGame`: `games_played`/`games_won` (sole-survivor rule)/damage dealt/`highest_life_total_achieved` (replayed from the action log)/eliminations — incremental upsert (`ON CONFLICT DO UPDATE`)
- [x] Real `GetUserStats`/`GetDeckStats` (zeros if never played, not 404)
- [x] `GetDeckStats` ownership check
- [x] `GetPlaygroupStats` (live aggregation, no summary table)
- [x] Integration tests (winner/no-winner, accumulation, ownership, aggregation)
- [x] `openapi.yaml` updated
- [x] **`FinishGame` had no `AND status = 'active'` guard, so concurrent calls double-counted statistics** (found and fixed 2026-08-01). `FinishGame`/`StartGame` now guard their `UPDATE` with the expected current status (`AND status = 'active'`/`'pending'`); a losing concurrent call now affects 0 rows and maps to the existing `409` error instead of silently succeeding twice. Regression test `TestFinishGame_Concurrent_OnlyOneSucceedsAndStatsAreNotDoubleCounted` (8 concurrent callers, real Postgres, `-race`) locks this in. `RecalculateForGame` itself is still purely additive (not a true from-scratch recompute) — that's now safe because the guard guarantees it only ever runs once per game, but a genuine `recalculate-stats` command remains unimplemented, see [ADR-0011](../decisions/0011-estrategia-migraciones-y-recalculo-estadisticas.md). See DECISIONS-LOG.md for the full before/after.

### Infra / configuration
- [x] `internal/config`: centralized env var config, fails fast on missing `JWT_SECRET`/`CORS_ALLOWED_ORIGINS` when `APP_ENV=production`
- [x] `.env.example` + `docker-compose.yml` aligned on the same variables
- [x] CORS via `CORS_ALLOWED_ORIGINS` (open by default in dev) — see [ADR-0003](../decisions/0003-cors-permisivo-en-dev.md)
- [x] `docker-compose.yml` centralized at the repo root: `db` (healthcheck)/`api`/`web`
- [x] PostgreSQL pinned to 18 everywhere (`docker-compose.yml`, `backend-ci.yml`)
- [x] Goose migrations applied automatically on binary startup (`internal/common/migrate.go`, session-level advisory lock for multi-replica safety)
- [x] Render deployment groundwork documented (`backend/README.md`) — see [ADR-0015](../decisions/0015-infraestructura-de-despliegue.md)
- [x] **TCP_NODELAY on accepted connections** (`cmd/api/listener.go`, found and fixed 2026-08-05): fasthttp (Fiber's engine) doesn't disable Nagle's algorithm itself, so a keep-alive connection reused across requests stalled ~40ms per request (Nagle plus the peer's delayed ACK) before the server saw a split header/body write. `main.go` now wraps the raw `net.Listener` to set `SetNoDelay(true)` on every accepted `*net.TCPConn`. Confirmed locally: ~45ms → ~5-7ms for a POST with a JSON body over a reused connection.

## Stage 2: Database

- [x] Initial DBML schema (`docs/database/schema.dbml`) + initial migration (`00001_initial_schema.sql`)
- [x] Explicit indexes beyond PKs (`decks.moxfield_id`, `game_actions.game_id`, `game_players.game_id`, migration `00003`)
- [x] `CHECK` constraints on `games.status`/`game_actions.action_type` (migration `00004`)
- [x] `users.google_id`/nullable `password_hash` + `refresh_tokens` (migration `00002`) + DB-level CHECK
- [x] Visual ER diagram (`docs/diagrams/er-diagram.md`, Mermaid, validated by rendering)
- [x] Future migration strategy — [ADR-0011](../decisions/0011-estrategia-migraciones-y-recalculo-estadisticas.md)
- [x] **`schema.dbml` was out of date**: the `deck_resync_jobs` table (migration `00013_deck_resync_jobs.sql`) was entirely missing from `schema.dbml` — found in a 2026-08-01 review by compiling the DBML to SQL and diffing it against the real migrated schema. Found already fixed on `main` when revisited on 2026-08-09 (the table, its status CHECK, and its partial unique index are all correctly documented) — fixed in an earlier pass not captured by this checklist item at the time. The underlying gap this item also flagged — `docs-ci.yml`'s Spectral/`dbml2sql` checks only validate that the DBML is well-formed, not that it matches the migrations, so nothing currently catches this class of drift automatically — is still real and unaddressed; a CI check that applies goose and compiles the DBML and diffs `information_schema` (see README.md §3, "edit `schema.dbml` first") remains a good idea, just not done.

## Stage 3: API (OpenAPI contract)

- [x] OpenAPI 3.1 skeleton with the main paths
- [x] Complete `requestBody`/schemas for auth, decks, games, playgroups, statistics, and `POST /auth/google`
- [x] `/statistics/playgroup/{id}` really implemented (previously hardcoded `501`)
- [x] Cursor-based pagination on `/games`/`/decks`/`/playgroups` listings (`internal/common/pagination.go`; `/playgroups` added 2026-08-08, opt-in via `cursor`/`limit` so the unpaginated response shape existing clients (Android, web) already parse is unchanged when neither is given — see DECISIONS-LOG.md). `GET /games/{id}/timeline` deliberately left unpaginated: a single game's action count is bounded (unlike account-wide history), and `GameViewModel.replayCommanderDamageUseCase` on Android needs the *complete* timeline to reconstruct commander-damage state — a paginated fetch there risks silent data corruption (an incomplete replay) for a highly speculative benefit, so it wasn't worth the added client complexity.
- [x] Spec linting with Spectral (`.spectral.yaml`, runs in `docs-ci.yml`) — 0 errors, ~100 pre-existing warnings (missing `operationId`/`tags`/`description`), non-blocking

## Stage 4b: Web Client (Nuxt)

- [x] Second client decision: Nuxt 4 + Tailwind, SSR, decoupled (REST-only) — see [ADR-0004](../decisions/0004-web-client-nuxt.md)
- [x] Initial skeleton: login (password + Google), `/` protected dashboard, `useAuth.ts`, `auth.global.ts` route guard
- [x] Full auth flow: registration, `httpOnly` cookies via Nitro BFF, automatic session refresh, Moxfield import, statistics pages, base app structure, ESLint + typecheck
- [x] `web-ci.yml`: eslint/typecheck/build + hadolint
- [x] Playgroup screens (list, detail, rename, add member, history)
- [x] Account Settings screen (password change, Moxfield username, bulk import trigger)
- [x] Internationalization with `@nuxtjs/i18n` (es/en/ca) — see [ADR-0014](../decisions/0014-internacionalizacion-web.md)
- [x] Custom Google Sign-In button styling; default `<title>`/`titleTemplate`
- [x] UI fixes: header opacity on scroll, deck image layout + "view on Moxfield," editable username + hide password form on passwordless accounts
- [x] Bulk Moxfield import button always shown in Settings (was gated behind `runtimeConfig.public.enableBulkMoxfieldImport`, removed 2026-08-03 once `ListDecksByUsername` was confirmed working end to end against the real Moxfield API from the actual Go client, see Stage 8)
- [x] Create games from the web: 100% browser-local life tracker (`useLocalGame.ts`, `app/pages/play.vue`), no backend persistence
- [x] Real Fullscreen API for the web life tracker (`useFullscreen.ts`)
- [ ] Visual polish pass on the web client — no detailed scope yet
- [ ] Statistics screens show zeros until there are real finished games for that user/deck/group (expected behavior, not a bug — noted here per the original scope agreement)

## Stage 4: Android Client (base)

- [x] Project initialized: Compose, Material 3, Navigation, Hilt, Room, Retrofit, kotlinx.serialization
- [x] Base theming + navigation with routes
- [x] Complete local life tracker (Room-persisted): setup, tracker, commander damage, history — 100% local, no backend
- [x] Pre-game screen: turn draw randomizer + per-player mulligan counter
- [x] Navigation flow defined (`LoginRoute → DashboardRoute → ... `), later connected to the real backend
- [x] Real auth: Credential Manager + Google Identity Services, `LoginViewModel` against `POST /auth/login`/`/auth/google`, session in `SessionManager` (DataStore), real logout
- [x] Domain layer: `domain/repository/` interfaces + `*Impl`, Hilt `RepositoryModule`, `domain/usecase/` (`ResolveGameOutcomeUseCase`, `ReplayCommanderDamageUseCase`, `LoadStatisticsUseCase`) each with its own test — auth (`LoginViewModel`/`RegisterViewModel`/`SettingsViewModel`/etc.) deliberately stays outside it, still injects `AuthApi`/`SessionManager` directly
- [x] Real repositories in `data/repository/` (decide Room vs. backend)
- [x] `CommanderApi.kt` with 15 real endpoints; separate `AuthApi.kt`
- [x] `GameState.kt`: life/turn/commander-damage/poison for 2-6 players
- [x] Full visual redesign (violet/dark palette, `AppComponents.kt`, landscape seat grid, pause overlay, game summary) + real poison counter + registration flow (`RegisterScreen`)
- [x] Internationalization: literals extracted to `strings.xml`, copy moved from Argentine "voseo" to Spain's "tuteo"; `values-en`/`values-ca` added; manual language switcher in Settings (`AppCompatDelegate.setApplicationLocales`)
- [x] Account Settings screen (username, Moxfield username, password change — gated on `has_password`)
- [ ] Improve the Android life tracker — no detailed scope yet (new life-change mechanic, elimination animation, commander-damage flow polish)
- [ ] Improve `PreGameScreen` — no detailed scope yet (single "Play" button instead of draw+start, deck thumbnails in the picker)

## Stage 5: Android ↔ Backend Integration

- [x] `AuthInterceptor`/`AuthAuthenticator` (Bearer token + refresh-on-401 retry)
- [x] Session persistence with DataStore (`SessionManager`)
- [x] **`SessionManager.refreshAccessToken()` had no concurrency dedup** (found and fixed 2026-08-01): several requests 401ing at once (e.g. `LoadStatisticsUseCase`'s parallel `async`/`awaitAll` calls, all sharing one expired access token) each called `runBlocking { refreshAccessToken() }` from their own OkHttp dispatcher thread with no synchronization — since the backend **rotates** the refresh token on every use (ADR-0001) and treats reuse of an already-rotated-away token as theft (revokes the whole session family, see the 2026-08-01 backend security-audit entries in DECISIONS-LOG.md), concurrent refreshes could log the user out on every device over a timing coincidence, not an actual security event. Fixed with a new, Context-free, independently unit-tested `core/util/SingleFlight.kt` (collapses concurrent callers into one real execution, same shape as the web BFF's already-existing `inFlightRefresh` map in `server/utils/backend.ts`) — `SessionManager` now runs `refreshAccessToken()` through it. **Not verified with `./gradlew`** (this sandbox still blocks `dl.google.com`), but the new logic was compiled and its tests actually run — not just reasoned about — with a standalone Kotlin compiler + JUnit fetched from Maven Central, bypassing Gradle/AGP entirely.
- [x] Best-effort mirroring of the local seat against the real backend (`GameRepository.bootstrapRemoteGame`, additive — never blocks the local tracker)
- [x] Casual/Group mode in `PlayerSetupScreen` with real proxy-join (seats assigned to real playgroup members, `RemoteGameSession.seatPlayerIds`)
- [x] Second device joins an already-created remote game (`JoinGameScreen`/`JoinGameViewModel`, `JoinedGameTrackerRoute`)
- [x] Live reconciliation of other seats' actions over the WebSocket (`ownedSeatIds`, read-only rendering for non-local seats, `game_finished` broadcast handling)
- [x] Room as an offline-first cache for own decks (`DeckEntity`/`DeckDao`, network-first with cache fallback)

## Stage 6: Synchronization (WebSocket)

- [x] Message protocol design — [ADR-0005](../decisions/0005-websocket-protocol.md) (envelope, auth-as-first-message, explicit out-of-scope list: replay guarantees, heartbeat, multi-process pub-sub)
- [x] Server implementation: in-memory `Hub`, `Client`, `GET /api/v1/ws/games/:id`, `games`/`game-actions` broadcast via interfaces
- [x] WebSocket client on Android: `GameSocketClient`, reconnect with exponential backoff (`GameSocketReconnect.kt`)
- [ ] **No ping/pong or read-deadline heartbeat anywhere in `internal/websocket`** (found in a 2026-08-01 review): a client that drops off the network without a clean TCP close (the common case for a phone losing signal) leaves its `Hub` entry, `readLoop`, and `writePump` goroutines running forever. ADR-0005 documents this as explicitly out of scope, but it's worth revisiting given how the app is actually used. Not fixed in this pass.

## Stage 7: Statistics

- [x] Real recalculation on game finish (see Stage 1 for the caveat about it being additive-only)
- [x] Aggregation queries (`UpsertUserStatistics`/`UpsertDeckStatistics`)
- [x] Real `GetUserStats`/`GetDeckStats`/`GetPlaygroupStats`
- [x] Statistics UI on Android (`StatisticsScreen`/`StatisticsViewModel`, same 3 endpoints as the web)
- [x] Head-to-head opponent stats (`GET /statistics/opponents`), per-playgroup game counts in one request (`GET /statistics/playgroups`, replaces the web client's old per-group `GetPlaygroupStats` loop), and a keyset-paginated finished-games history (`GET /statistics/games`) — all live-computed, no summary table (2026-08-05)
- [x] Web `statistics.vue`: decks/games tabs, deck sorting (recent/win rate/games played), "most played group" and "archenemy" cards from the new endpoints
- [x] Android: `FinishedGamesViewModel` + `StatisticsScreen` updated for the same finished-games history

## Stage 8: Moxfield Import

- [x] Public/unofficial Moxfield API investigated (`GET https://api2.moxfield.com/v3/decks/all/{publicId}`, needs a browser-like `User-Agent`)
- [x] HTTP client (`internal/moxfield`, `GetDeck`/`ExtractPublicID`)
- [x] `internal/sync` real and deliberately synchronous (no job queue) — see `internal/sync/service.go`'s doc for why
- [x] `POST /decks/import/moxfield` end-to-end (404/400 handled)
- [x] Imported deck image (`decks.image_url`, art crop from Moxfield's `main.id`)
- [x] Retries + backoff against Moxfield (3 attempts, `Retry-After` respected); errors mapped to a clean 503
- [x] `users.moxfield_username` field + `PATCH /users/{id}`
- [x] **Background bulk import of all of a user's decks — `MoxfieldClient.ListDecksByUsername` implemented and confirmed working end to end against the real Moxfield API** (2026-08-02/03): `internal/moxfieldimport` (job table, one-active-job-per-user, progress polling) is real and tested against a mocked client. `ListDecksByUsername` calls Moxfield's undocumented `GET /v2/decks/search-sfw` (paginated, filtered by `authorUserNames`), reverse-engineered from a third-party client ([Aleqsd/moxfield-api](https://github.com/Aleqsd/moxfield-api)) since this sandbox's network policy still blocks `api2.moxfield.com` directly. The assumed response shape (`data[].publicId`, `totalPages`) was manually confirmed against the real endpoint (`GET .../v2/decks/search-sfw?authorUserNames=vansid&...`, hit from a phone browser, no Cloudflare challenge, no `Referer` needed) — matches the Go structs exactly. Cloudflare acceptance from the actual Go client (non-browser `User-Agent`) confirmed too: a real `POST /moxfield-import` run against `vansid` from the local docker-compose stack completed end to end (`total_decks=48, imported_count=47, failed_count=1`). The web client's `enableBulkMoxfieldImport` opt-in flag was removed as a result — the button is unconditionally shown now (see Stage 3's web tasks).
- [x] **`StartImport` used to resolve the Moxfield deck list synchronously before returning, tying the HTTP request (and the browser tab that made it) to however long Moxfield's search took** (found and fixed 2026-08-03, reported against the web client's re-enabled import button): `StartImport` now only creates the job (`pending`, `total_decks` null) and returns; `runImport`'s background goroutine does both the listing AND the per-deck import, marking the job `in_progress` (with `total_decks` known) once listing succeeds, or straight to `failed` with `error_message` set if it doesn't. Regression test `TestStartImport_ReturnsImmediately_DoesNotWaitForMoxfieldListing` (`internal/moxfieldimport/service_test.go`) locks in that `StartImport` returns before a slow (background) listing call finishes. `docs/api/openapi.yaml` and the web client (`settings.vue`, polls through `pending` too now) updated to match.
- [x] **Neither the single-deck import nor the bulk import checked for an existing deck before inserting, so re-importing (retrying a bulk import, or importing the same Moxfield URL through both paths) created duplicate decks** (found and fixed 2026-08-03, reported after inspecting the real Supabase DB — 0 duplicates found there today, `moxfield_import_jobs` was empty, so this was a latent gap rather than already-corrupted data). Fix: partial unique index `decks_user_id_moxfield_id_unique_idx` on `(user_id, moxfield_id) WHERE moxfield_id IS NOT NULL` (migration `00015_decks_unique_moxfield_id_per_user.sql`; NOT globally unique on `moxfield_id` alone — the same public deck can legitimately be imported by different users). `ImportFromMoxfield` (`internal/decks/service.go`) now maps the constraint violation to `ErrDeckAlreadyImported` → `409`; the bulk import's `runImport` (`internal/moxfieldimport/service.go`) treats that specific error as a success (`imported_count`, not `failed_count`) since the desired end state — the deck being in the user's collection — already holds. `docs/api/openapi.yaml` and the web client's `moxfieldImportError` updated to match.
- [x] Resync all already-imported decks, async (`internal/deckresync`, same pattern as the bulk import, but for decks the user already has)
- [x] **A crash/restart mid-import or mid-resync used to leave the job stuck `in_progress` forever**, and the partial-unique-index (one active job per user) then blocked starting a new one — no retry, no reaper (found 2026-08-01, fixed 2026-08-08). Fix: `moxfieldimport.ReapStaleJobs`/`deckresync.ReapStaleJobs` (`ReapStaleImportJobs`/`ReapStaleResyncJobs` sqlc queries) mark any job still `pending`/`in_progress` as `failed` with an explanatory `error_message`, called once at startup (`cmd/api/main.go: reapStaleBackgroundJobs`, right after the DB pool connects) — safe unconditionally in the current single-instance deployment, since a freshly starting process can't have any of these jobs actually in flight yet. Regression tests `TestReapStaleJobs_MarksPendingAndInProgressAsFailed`/`TestReapStaleJobs_MarksInProgressAsFailed` lock in that the reap both flips the stuck row to `failed` and frees the active-job constraint for a new attempt. Verified locally in Docker (no Go toolchain in this sandbox otherwise): `sqlc/sqlc:1.27.0` regenerated `query.sql.go`/`querier.go` byte-for-byte identical to what was hand-written, `golang:1.25-alpine` ran `go build`/`go vet`/`golangci-lint run` clean (only pre-existing repo-wide `gofmt` CRLF noise from this Windows checkout's `core.autocrlf=true`, unrelated to this change), and the full suite (`go test -race -cover -p 1 ./...`, against a throwaway `postgres:18-alpine` container with goose migrations applied) passed for every package.

## Stage 9: Social — friends, groups and tournaments

- [ ] Friends system: send/accept/reject requests, friends list — distinct from `playgroups` (game groups, not friendship relations)
- [ ] Groups beyond the existing `playgroups`, or extend them — not needed for tournaments in the end (see below), still open for any future friends/groups work
- [x] **Standalone Swiss-format Commander tournaments** (2026-08-09, pulled forward ahead of the rest of this stage — see [ADR-0016](../decisions/0016-swiss-tournament-format.md) for the full design and the alternatives ruled out): `backend/internal/tournaments` (migration `00016_tournaments.sql`, five new tables, none touching `games`/`game_players`), organizer creates a tournament and gets a join code, app users self-register with one of their own decks (`POST /tournaments/join`), the organizer adds guests with no account (`POST /tournaments/{id}/participants`), `POST /tournaments/{id}/start` locks the roster (valid 3-4 player table split required, min 3, max 4 per table) and seats round 1 via a greedy repeat-avoiding pairing heuristic (`pairing.go`), round count is computed automatically from the roster size, the organizer records each table's finish order (`POST /tournaments/{id}/tables/{tableId}/result`, scoring 2/1/0/0) and advances rounds (`POST /tournaments/{id}/rounds/next`) until the tournament finishes, and `GET /tournaments/lookup?code=` is the "enter the code" read-only lookup of a participant's current table. Deliberately **not** wired into the live `games`/`game_players`/WebSocket life tracker or into Android — both explicitly out of scope for this pass, see the ADR. Full test coverage (`pairing_internal_test.go` for the pairing math, `service_test.go` including a full 3-round tournament lifecycle test against real Postgres) and web UI (`web/app/pages/tournaments/`).

## Cross-cutting (quality, infrastructure, security)

- [x] Backend integration tests (real Postgres) across all 9 modules with queries
- [x] Android unit tests (JUnit + `kotlinx-coroutines-test`, hand-written fakes, no mocking framework)
- [x] CI/CD: `backend-ci.yml`, `android-ci.yml`, `web-ci.yml`, `docs-ci.yml` — all four follow the same always-reports-a-check `changes`/`dorny-paths-filter` pattern
- [x] Branch protection on `main` (8 required checks — `web-ci.yml`'s checks aren't in that list yet, see README.md §6 for the `hadolint` name-collision caveat)
- [x] `golangci-lint` at 0 issues repo-wide (config migrated to v2, pinned to `v2.12.2`)
- [x] Real wiring of every `service.go` to its sqlc `Queries` — no dummy modules left
- [x] Consistent error handling (`internal/common/errors.go: DomainError`/`MapError`, no raw internal errors leaked to clients)
- [x] Rate limiting on auth endpoints (20 req/min/IP, in-process memory — needs a shared `Storage` if ever deployed with >1 replica)
- [x] **Dependency updates found in a 2026-08-08 audit, applied 2026-08-09** — Go backend was already fully current (no action needed). Web (`web/package.json`): `npm update` applied the pending minor/patch bumps (`nuxt` 4.5.1→4.5.2, `vue` 3.5.40→3.5.41, `@nuxtjs/i18n` 10.5.0→10.6.0, `@nuxt/eslint` 1.16.0→1.17.0, `eslint` 10.8.0→10.8.1, `vue-tsc` 3.3.8→3.3.9, `@types/node` 26.1.2→26.2.0) within their existing `^` ranges — TypeScript's major jump (`6.0.3`→`7.0.2`) deliberately still deferred (suspected incompatible with the current Vue 3/Nuxt toolchain, `npm update` leaves it alone on its own since it's outside the caret range anyway). **Verified with a real `npm run build`** (not just lint/typecheck), Docker/`node:22-alpine`, production bundle succeeded. Android (`android/gradle/libs.versions.toml`): `navigation-compose` moved off the old alpha (`2.8.0-alpha07` → `2.9.8` stable); also bumped `kotlinx-coroutines` (`1.7.3`→`1.11.0`), `androidx-credentials` (`1.3.0`→`1.6.0`), `androidx-lifecycle-runtime-ktx` (`2.7.0`→`2.11.0`), `androidx-core-ktx` (`1.13.1`→`1.19.0`), `androidx-activity-compose` (`1.9.0`→`1.13.0`), `kotlin` (`2.2.10`→`2.4.10`), `hilt-navigation-compose` (hardcoded `1.2.0`→`1.4.0`), `androidx-datastore-preferences` (`1.1.1`→`1.2.1`), `coil` (`3.4.0`→`3.5.0`), `googleid` (`1.1.1`→`1.2.0`), `appcompat` (`1.7.0`→`1.7.1`); `agp`/`hilt`/`room`/`retrofit`/`okhttp`/`compose-bom` were already current. The KSP Gradle plugin was also bumped (`2.2.10-2.0.2` → `2.3.11`, KSP's newest release) to keep pace with the Kotlin bump. **Verified 2026-08-09** with a real local `./gradlew build` (JDK 21 + Android SDK) — KSP `2.3.11`/Kotlin `2.4.10` turned out to be compatible, `kspDebugKotlin`/`kspReleaseKotlin` ran clean; that was a false alarm. Two real breakages did surface, both fixed on `session-0908` (commit `d060709`): (1) AGP 9.3.1 treats the legacy `android { kotlinOptions { jvmTarget = "1.8" } }` setter as a hard script-compilation error, not a warning — migrated to the `kotlin { compilerOptions { jvmTarget.set(...) } }` DSL; (2) `navigation-compose 2.9.8` graduated type-safe navigation out of experimental and removed `ExperimentalSafeArgsApi` entirely, so the `@OptIn` in `AppNavigation.kt` no longer resolved — removed it (feature works unconditionally now). Fixing (2) also required raising the project's own JVM target from 1.8 to 11 (`compileOptions`/`compilerOptions`), since several of the bumped androidx/Compose libraries now ship JVM 11 bytecode that can't be inlined into 1.8-target code — safe given `minSdk 26`. `./gradlew build` now passes clean end to end: compile (debug+release), unit tests, lint, `assembleDebug`, `assembleRelease`.
- [ ] Record ADRs in `docs/decisions/` as new non-trivial technical decisions are made — ongoing practice, not a one-time task
- [x] Cleanup of empty/residual folders (`docker/`, `scripts/` never existed); `.github/dependabot.yml` (4 ecosystems, weekly)
- [x] CI path filters fixed so a README-only change doesn't trigger the full backend/Android/web battery
- [x] Documentation restructuring: split this file's narrative into [DECISIONS-LOG.md](DECISIONS-LOG.md), so a new session doesn't have to load the full project history just to check status (2026-08-01, see DECISIONS-LOG.md)
- [x] Deployment infrastructure stack decided — [ADR-0015](../decisions/0015-infraestructura-de-despliegue.md) (Render + Vercel + Supabase); `render.yaml`/a deploying CI workflow remain a future improvement, deployment is manual today

---

## Suggested order of work

1. ~~Quick hygiene: minimal ADRs for what's already decided~~ — done.
2. ~~Real auth (Stage 1): JWT + bcrypt + middleware~~ — done. Only the external manual step remains: Google Cloud OAuth credentials.
3. ~~Connect services to the real database (Cross-cutting)~~ — done, backend 100% wired.
4. ~~Real statistics (Stage 7): recalculation on game finish~~ — done, but see the Stage 1 caveat on double-counting under concurrent `FinishGame` calls.
5. ~~Complete the OpenAPI contract~~ (Stage 3) — done; `/games/{id}/timeline` is the one listing deliberately left unpaginated (see Stage 3 entry above for why).
6. ~~Android: real auth (Stage 4-5)~~ — done. Pending the external Google Cloud Web Client ID to test the Google flow end to end.
7. ~~WebSocket (Stage 6)~~ — server + Android client done; no heartbeat (see Stage 6 gap above).
8. **Moxfield integration** (Stage 8) — single-deck import and resync work; bulk import by username still blocked on confirming the real Moxfield endpoint.
9. **Tests + CI** — in place and kept current as modules are built, not left for the end.
