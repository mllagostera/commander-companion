# Commander Companion — Task list

Compact status checklist, organized by the **Stages** defined in [ROADMAP.md](ROADMAP.md). This document is the source of truth for day-to-day progress: the ROADMAP explains the *what* and the *why*, this file tracks the *status*.

The full narrative behind any item — what changed, why, gotchas hit, how it was verified, dates, PRs, exact user requests — lives in **[DECISIONS-LOG.md](DECISIONS-LOG.md)**, organized by the same stages. This file only keeps what's needed to know current status at a glance; read the log when you need the "why" or the history behind a decision.

**How to keep it up to date:**
- Mark `[x]` when a task is finished and functionally complete (not just "it compiles").
- If something is halfway done (scaffolding without real logic, stub, dummy data), leave it as `[ ]` and add a short note in parentheses explaining what's missing.
- Add new tasks as they come up; don't delete completed ones, they're useful history.
- Keep each line short (status + what + a pointer to the file/module). Put narrative, rationale, and verification detail in DECISIONS-LOG.md instead, dated, under the matching stage — don't let it accumulate here again.
- Update the "Last reviewed" date every time the real state of the code is audited, and add the corresponding entry to DECISIONS-LOG.md.

**Last reviewed:** 2026-08-01 — see [DECISIONS-LOG.md](DECISIONS-LOG.md) for the full history of audits and decisions up to that date, including this restructuring itself.

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

## Stage 2: Database

- [x] Initial DBML schema (`docs/database/schema.dbml`) + initial migration (`00001_initial_schema.sql`)
- [x] Explicit indexes beyond PKs (`decks.moxfield_id`, `game_actions.game_id`, `game_players.game_id`, migration `00003`)
- [x] `CHECK` constraints on `games.status`/`game_actions.action_type` (migration `00004`)
- [x] `users.google_id`/nullable `password_hash` + `refresh_tokens` (migration `00002`) + DB-level CHECK
- [x] Visual ER diagram (`docs/diagrams/er-diagram.md`, Mermaid, validated by rendering)
- [x] Future migration strategy — [ADR-0011](../decisions/0011-estrategia-migraciones-y-recalculo-estadisticas.md)
- [ ] **`schema.dbml` is out of date**: the `deck_resync_jobs` table (migration `00013_deck_resync_jobs.sql`) is entirely missing from `schema.dbml` — found in a 2026-08-01 review by compiling the DBML to SQL and diffing it against the real migrated schema. `docs-ci.yml`'s Spectral/`dbml2sql` checks only validate that the DBML is well-formed, not that it matches the migrations — nothing currently catches this class of drift. Fix: add the `deck_resync_jobs` table to `schema.dbml`, and consider a CI check that applies goose + compiles the DBML and diffs `information_schema` (see README.md §3, "edit `schema.dbml` first").

## Stage 3: API (OpenAPI contract)

- [x] OpenAPI 3.1 skeleton with the main paths
- [x] Complete `requestBody`/schemas for auth, decks, games, playgroups, statistics, and `POST /auth/google`
- [x] `/statistics/playgroup/{id}` really implemented (previously hardcoded `501`)
- [x] Cursor-based pagination on `/games`/`/decks` listings (`internal/common/pagination.go`); `GET /games/{id}/timeline` and playgroup listings remain unpaginated
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
- [x] Bulk Moxfield import button gated behind `runtimeConfig.public.enableBulkMoxfieldImport` (default off — backend endpoint still a stub)
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

## Stage 8: Moxfield Import

- [x] Public/unofficial Moxfield API investigated (`GET https://api2.moxfield.com/v3/decks/all/{publicId}`, needs a browser-like `User-Agent`)
- [x] HTTP client (`internal/moxfield`, `GetDeck`/`ExtractPublicID`)
- [x] `internal/sync` real and deliberately synchronous (no job queue) — see `internal/sync/service.go`'s doc for why
- [x] `POST /decks/import/moxfield` end-to-end (404/400 handled)
- [x] Imported deck image (`decks.image_url`, art crop from Moxfield's `main.id`)
- [x] Retries + backoff against Moxfield (3 attempts, `Retry-After` respected); errors mapped to a clean 503
- [x] `users.moxfield_username` field + `PATCH /users/{id}`
- [ ] **Background bulk import of all of a user's decks — `MoxfieldClient.ListDecksByUsername` implemented but unverified live** (2026-08-02): `internal/moxfieldimport` (job table, one-active-job-per-user, progress polling) is real and tested against a mocked client. `ListDecksByUsername` now calls Moxfield's undocumented `GET /v2/decks/search-sfw` (paginated, filtered by `authorUserNames`), reverse-engineered from a third-party client ([Aleqsd/moxfield-api](https://github.com/Aleqsd/moxfield-api)) since this sandbox's network policy still blocks `api2.moxfield.com` directly. Unit-tested against a fake HTTP server (pagination, retries, 5xx exhaustion), but never exercised against the real endpoint — needs confirming from an environment with network access (response shape, whether Cloudflare requires more than the existing User-Agent/Referer) before flipping the web client's `enableBulkMoxfieldImport` on by default.
- [x] Resync all already-imported decks, async (`internal/deckresync`, same pattern as the bulk import, but for decks the user already has)
- [ ] **A crash/restart mid-import or mid-resync leaves the job stuck `in_progress` forever**, and the partial-unique-index (one active job per user) then blocks starting a new one — no retry, no reaper. Documented as an accepted gap when built; worth revisiting given ADR-0015's Render free tier sleeps the service between requests.

## Stage 9: Social — friends, groups and tournaments

- [ ] Friends system: send/accept/reject requests, friends list — distinct from `playgroups` (game groups, not friendship relations)
- [ ] Groups beyond the existing `playgroups`, or extend them, depending on the tournament flow
- [ ] Tournament creation: among friends, among groups, or open registration
- [ ] Define tournament format (bracket, swiss, league, etc.) and its data model before implementing

## Cross-cutting (quality, infrastructure, security)

- [x] Backend integration tests (real Postgres) across all 9 modules with queries
- [x] Android unit tests (JUnit + `kotlinx-coroutines-test`, hand-written fakes, no mocking framework)
- [x] CI/CD: `backend-ci.yml`, `android-ci.yml`, `web-ci.yml`, `docs-ci.yml` — all four follow the same always-reports-a-check `changes`/`dorny-paths-filter` pattern
- [x] Branch protection on `main` (8 required checks — `web-ci.yml`'s checks aren't in that list yet, see README.md §6 for the `hadolint` name-collision caveat)
- [x] `golangci-lint` at 0 issues repo-wide (config migrated to v2, pinned to `v2.12.2`)
- [x] Real wiring of every `service.go` to its sqlc `Queries` — no dummy modules left
- [x] Consistent error handling (`internal/common/errors.go: DomainError`/`MapError`, no raw internal errors leaked to clients)
- [x] Rate limiting on auth endpoints (20 req/min/IP, in-process memory — needs a shared `Storage` if ever deployed with >1 replica)
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
5. **Complete the OpenAPI contract** (Stage 3) — request/response wiring done; some listings remain unpaginated.
6. ~~Android: real auth (Stage 4-5)~~ — done. Pending the external Google Cloud Web Client ID to test the Google flow end to end.
7. ~~WebSocket (Stage 6)~~ — server + Android client done; no heartbeat (see Stage 6 gap above).
8. **Moxfield integration** (Stage 8) — single-deck import and resync work; bulk import by username still blocked on confirming the real Moxfield endpoint.
9. **Tests + CI** — in place and kept current as modules are built, not left for the end.
