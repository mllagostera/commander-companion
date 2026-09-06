# Commander Companion — Task list

Compact status checklist, organized by the **Stages** defined in [ROADMAP.md](ROADMAP.md). This document is the source of truth for day-to-day progress: the ROADMAP explains the *what* and the *why*, this file tracks the *status*.

The full narrative behind any item — what changed, why, gotchas hit, how it was verified, dates, PRs, exact user requests — lives in **[DECISIONS-LOG.md](DECISIONS-LOG.md)**, organized by the same stages. This file only keeps what's needed to know current status at a glance; read the log when you need the "why" or the history behind a decision.

**How to keep it up to date:**
- Mark `[x]` when a task is finished and functionally complete (not just "it compiles").
- If something is halfway done (scaffolding without real logic, stub, dummy data), leave it as `[ ]` and add a short note in parentheses explaining what's missing.
- Add new tasks as they come up; don't delete completed ones, they're useful history.
- Keep each line short (status + what + a pointer to the file/module). Put narrative, rationale, and verification detail in DECISIONS-LOG.md instead, dated, under the matching stage — don't let it accumulate here again.
- Update the "Last reviewed" date every time the real state of the code is audited, and add the corresponding entry to DECISIONS-LOG.md.

**Last reviewed:** 2026-08-17 — see [DECISIONS-LOG.md](DECISIONS-LOG.md) for the full history of audits and decisions up to that date, including this restructuring itself.

---

## Stage 0: Functional definition

- [x] Roadmap (`docs/roadmap/ROADMAP.md`)
- [x] Architecture and principles document (`docs/architecture/ARCHITECTURE.md`)
- [x] Detailed use cases — [`docs/ux/use-cases.md`](../ux/use-cases.md) (2026-08-10: extended past the original 5 game-loop use cases with 3 more — decks/Moxfield, playgroups, tournaments — that were missing entirely)
- [x] Android wireframes — [`docs/ux/wireframes.md`](../ux/wireframes.md)
- [x] Web client screenshot gallery — [`docs/ux/screenshots.md`](../ux/screenshots.md) (2026-08-09; real screenshots of every flow, captured via Playwright against a local backend+DB, not mockups)
- [x] Flow/state diagrams — [`docs/diagrams/game-state-machine.md`](../diagrams/game-state-machine.md), [`docs/diagrams/android-navigation-flow.md`](../diagrams/android-navigation-flow.md)
- [x] Foundational ADRs (retroactive, inherited decisions): [0006](../decisions/0006-go-fiber-backend.md) Go+Fiber, [0007](../decisions/0007-postgresql.md) PostgreSQL, [0008](../decisions/0008-sqlc-goose.md) sqlc+goose, [0009](../decisions/0009-android-native-vs-crossplatform.md) native Android, [0010](../decisions/0010-modular-monolith-vs-microservices.md) modular monolith
- [x] Session ADRs: [0001](../decisions/0001-auth-jwt-refresh-token-strategy.md) auth strategy, [0002](../decisions/0002-google-sign-in.md) Google Sign-In, [0003](../decisions/0003-permissive-cors-in-dev.md) CORS in dev, [0004](../decisions/0004-web-client-nuxt.md) web client, [0005](../decisions/0005-websocket-protocol.md) WebSocket protocol

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
- [x] ADR: block login until email confirmed, Resend with dashboard templates — [ADR-0012](../decisions/0012-email-verification-resend.md)
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
- [x] `JoinGame`: self-join by default; `user_id` enables proxy-join, gated by shared playgroup membership — see [ADR-0013](../decisions/0013-proxy-join-and-action-authorization.md)
- [x] `GET /games?playgroup_id=`: full group history, membership-gated
- [x] `game-actions` wired: validates `action_type` against the fixed vocabulary, resolves actor/target as `game_players`, only in `active` games
- [x] Actions mutate real player state (life/poison/elimination); server-side auto-elimination at 0 life / 10 poison / 21 commander damage from one source
- [x] Integration tests covering the full state machine and every `action_type`
- [x] `CommanderDamage` tracked per attacker-defender pair (`commander_damage` table, migration `00007`); `RecordAction` runs inside a single transaction
- [x] `TurnStart`/`TurnEnd` set/clear `games.current_turn_player_id` (doesn't model turn order, only "whose turn is it now")
- [x] `openapi.yaml` updated for `/games/*`
- [x] **`GET /games?playgroup_id=` was N+1** (found and fixed 2026-09-02): `ListGamesForPlaygroup` ran one `ListGamePlayers` per game. New `ListGamePlayersForGames` (`game_id = ANY($1)`, same shape as `statistics.ListPlayersForGames`) fetches every seat in one round trip and the service groups them by game in memory. On a 40-game group: **41 queries → 2**, 16.7 → 5.9 ms, byte-identical response. Regression test `TestListGamesForPlaygroup_AttributesEachSeatToItsOwnGame` (games with 1/2/3 seats) covers the grouping the batching introduces
- [x] **`payload.amount` had no range/sign validation** (found in a 2026-08-01 review, see DECISIONS-LOG.md): negative `CombatDamage`/`CommanderDamage` silently healed the target, and a magnitude above `int32` range wrapped into a large negative number instead of erroring. **Fixed same day**: `CombatDamage`/`CommanderDamage` now require a strictly positive amount, and all `payload.amount` values are bounded to whole numbers with magnitude ≤ 1,000,000 (`ErrAmountOutOfRange`/`ErrAmountMustBePositive`, `internal/game-actions/service.go`); regression tests added. `openapi.yaml` updated with the new validation rules.

### Playgroups — game groups
- [x] `playgroups` wired: auto-join creator, membership-scoped listing/detail
- [x] `AddMember` validations (inviter is a member, target exists, not already a member)
- [x] Integration tests
- [x] Proxy-join via `game_players.added_by` + `RecordAction` actor-ownership authorization — see [ADR-0013](../decisions/0013-proxy-join-and-action-authorization.md)
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
- [x] **`FinishGame` had no `AND status = 'active'` guard, so concurrent calls double-counted statistics** (found and fixed 2026-08-01). `FinishGame`/`StartGame` now guard their `UPDATE` with the expected current status (`AND status = 'active'`/`'pending'`); a losing concurrent call now affects 0 rows and maps to the existing `409` error instead of silently succeeding twice. Regression test `TestFinishGame_Concurrent_OnlyOneSucceedsAndStatsAreNotDoubleCounted` (8 concurrent callers, real Postgres, `-race`) locks this in. `RecalculateForGame` itself is still purely additive (not a true from-scratch recompute) — that's now safe because the guard guarantees it only ever runs once per game, but a genuine `recalculate-stats` command remains unimplemented, see [ADR-0011](../decisions/0011-migration-strategy-and-statistics-recalculation.md). See DECISIONS-LOG.md for the full before/after.

### Infra / configuration
- [x] `internal/config`: centralized env var config, fails fast on missing `JWT_SECRET`/`CORS_ALLOWED_ORIGINS` when `APP_ENV=production`
- [x] `.env.example` + `docker-compose.yml` aligned on the same variables
- [x] CORS via `CORS_ALLOWED_ORIGINS` (open by default in dev) — see [ADR-0003](../decisions/0003-permissive-cors-in-dev.md)
- [x] `docker-compose.yml` centralized at the repo root: `db` (healthcheck)/`api`/`web`
- [x] PostgreSQL pinned to 18 everywhere (`docker-compose.yml`, `backend-ci.yml`)
- [x] Goose migrations applied automatically on binary startup (`internal/common/migrate.go`, session-level advisory lock for multi-replica safety)
- [x] Render deployment groundwork documented (`backend/README.md`) — see [ADR-0015](../decisions/0015-deployment-infrastructure.md)
- [x] **`GET /health` reports which build is answering** (2026-09-06): `commit` (full git SHA, from the linker → the binary's VCS stamp → `RENDER_GIT_COMMIT` → `unknown`) and `started_at`, on both the 200 and the 503 branch — `internal/config/buildinfo.go`, `internal/common/health.go`, `backend/Dockerfile`'s `GIT_COMMIT` build arg. Unblocks sequencing the deploy race below; does not fix it. `/health` also entered `openapi.yaml`, where it had never been. See [ADR-0020](../decisions/0020-build-provenance-in-health.md)
- [x] **TCP_NODELAY on accepted connections** (`cmd/api/listener.go`, found and fixed 2026-08-05): fasthttp (Fiber's engine) doesn't disable Nagle's algorithm itself, so a keep-alive connection reused across requests stalled ~40ms per request (Nagle plus the peer's delayed ACK) before the server saw a split header/body write. `main.go` now wraps the raw `net.Listener` to set `SetNoDelay(true)` on every accepted `*net.TCPConn`. Confirmed locally: ~45ms → ~5-7ms for a POST with a JSON body over a reused connection.

## Stage 2: Database

- [x] Initial DBML schema (`docs/database/schema.dbml`) + initial migration (`00001_initial_schema.sql`)
- [x] Explicit indexes beyond PKs (`decks.moxfield_id`, `game_actions.game_id`, `game_players.game_id`, migration `00003`)
- [x] `CHECK` constraints on `games.status`/`game_actions.action_type` (migration `00004`)
- [x] `users.google_id`/nullable `password_hash` + `refresh_tokens` (migration `00002`) + DB-level CHECK
- [x] Visual ER diagram (`docs/diagrams/er-diagram.md`, Mermaid, validated by rendering)
- [x] Future migration strategy — [ADR-0011](../decisions/0011-migration-strategy-and-statistics-recalculation.md)
- [x] **`schema.dbml` was out of date**: the `deck_resync_jobs` table (migration `00013_deck_resync_jobs.sql`) was entirely missing from `schema.dbml` — found in a 2026-08-01 review by compiling the DBML to SQL and diffing it against the real migrated schema. Found already fixed on `main` when revisited on 2026-08-09 (the table, its status CHECK, and its partial unique index are all correctly documented) — fixed in an earlier pass not captured by this checklist item at the time. The underlying gap this item also flagged — `docs-ci.yml`'s Spectral/`dbml2sql` checks only validate that the DBML is well-formed, not that it matches the migrations, so nothing currently catches this class of drift automatically — was real and unaddressed until **2026-09-02**, when it was closed: `docs-ci.yml`'s new `schema.dbml matches the migrations` job builds the schema twice in one Postgres — once with goose, once from the compiled DBML — and diffs `information_schema` (`.github/scripts/check-dbml-matches-migrations.sh`). It found 11 real drifts on its first run: 9 columns documented as nullable that are `NOT NULL`, and the two `00005_pagination_indices.sql` indexes missing entirely. It compares tables/columns/types/nullability only — not indexes or constraints, because DBML can't express partial indexes and `dbml2sql` renders unique constraints as unique indexes, so those comparisons report differences that aren't drift.

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
- [x] Internationalization with `@nuxtjs/i18n` (es/en/ca) — see [ADR-0014](../decisions/0014-web-internationalization.md)
- [x] Custom Google Sign-In button styling; default `<title>`/`titleTemplate`
- [x] UI fixes: header opacity on scroll, deck image layout + "view on Moxfield," editable username + hide password form on passwordless accounts
- [x] Bulk Moxfield import button always shown in Settings (was gated behind `runtimeConfig.public.enableBulkMoxfieldImport`, removed 2026-08-03 once `ListDecksByUsername` was confirmed working end to end against the real Moxfield API from the actual Go client, see Stage 8)
- [x] Create games from the web: 100% browser-local life tracker (`useLocalGame.ts`, `app/pages/play.vue`), no backend persistence
- [x] **Create decks by hand, with a Scryfall commander typeahead** (2026-08-16): `POST /decks` existed since Stage 1 but no client had ever called it — "Add deck" only opened the Moxfield import, so a deck that isn't on Moxfield couldn't be added at all. New `decks.create` modal (name + commander) with `CommanderSearch.vue`, an ARIA 1.2 combobox backed by a Nitro endpoint (`server/api/scryfall/commanders.get.ts`) that queries Scryfall's `is:commander` ordered by EDHREC rank. Picking a suggestion also attaches the card's `art_crop`, so hand-made decks get the same art as imported ones. Free text still accepted. The lookup runs in Nitro, not the browser: the app's CSP sets `connect-src 'self'`, Scryfall asks clients to send a `User-Agent` (which browsers can't set), and `/cards/search` returns full card objects that are trimmed to two fields before reaching the client. `img-src` gained `https://cards.scryfall.io`. The same pass added the missing blank-name/blank-commander validation to `CreateDeck` (the Moxfield import always filled both, so it had never been reachable).
- [x] Real Fullscreen API for the web life tracker (`useFullscreen.ts`)
- [ ] **Web accessibility audit against WCAG 2.2 AAA** (2026-08-10, manual code review across every page/component, `a11y/aaa-audit-fixes` branch): 5 critical + 6 serious + 4 of 6 moderate findings fixed — accessible names on the life tracker's life buttons (`PlayQuadrant.vue`), a global visible focus ring (`main.css`), toast/status announcements (`ToastHost.vue`/`SlowNavHint.vue`/`login.vue`), dialog semantics + focus trap + focus return on all 4 modals (new `useModalA11y.ts`), a skip-to-content link (`default.vue`), icon-only buttons and decorative glyphs given proper `aria-label`/`aria-hidden`, placeholder-only inputs labeled, a combobox pattern for the member search (`playgroups/[id].vue`), `SortSelect.vue` focus return, the statistics tabs completed to the APG pattern, the tracker's orientation lock documented as an intentional 1.3.4 exception, focus moved to the page heading after client-side navigation (`app.vue`), and `verify-email.vue` migrated off ad-hoc Tailwind colors onto the shared theme tokens. **Still open, deferred pending a design call rather than a technical blocker** (both require visibly changing existing colors): `--text-dim` (dark theme) and `--accent-link`/`--text-muted` (light theme) fall short of the 7:1 AAA contrast ratio (`main.css`); the life tracker's fixed black text over arbitrary per-player colors (`PlayQuadrant.vue`) can't guarantee AAA — or in some cases even AA — contrast against every color in the palette.
- [x] **Login splash stayed up only until the credentials call returned** (2026-09-02): `login.vue`'s `finally` cleared it before `navigateTo`, so the form was clickable again while the destination page loaded, and the Google button (a GSI iframe, no `disabled`) was never blocked at all. The overlay now lives until the page unmounts (`isNavigating`, never cleared on success), and the form is `inert` while busy
- [x] **Dashboard consumes `GET /statistics/dashboard`** (2026-09-02): `pages/index.vue` lost its whole client-side assembly (the per-group `/games` fan-out, the `listAllDecks` cursor chain, the local sorting/slicing/streak counting) — **30 backend requests → 1**, SSR TTFB 76 → 38 ms, hydration payload 25.7 → 5.1 KB on the 400-game fixture. `DeckArt`'s prop narrowed to `Pick<Deck, 'commander' | 'image_url'>`, the two fields it reads, which also removed the fake `Deck` literal `statistics.vue` had to build for it. One behaviour change: `won` is now the backend's sole-survivor rule, the same one the statistics screen uses, instead of the client's "not eliminated"
- [x] `onMounted(() => refresh())` in `pages/index.vue` no longer re-fetches during hydration (`nuxtApp.isHydrating` guard), so a hard load pays once instead of twice; it still refreshes when returning to the page in the same session, which is why it exists
- [ ] Reconsider shell-first rendering + skeletons for the dashboard **after** the endpoint lands — deliberately deferred, it addresses neither the payload nor the query count (see DECISIONS-LOG.md 2026-09-02)
- [ ] Visual polish pass on the web client — no detailed scope yet
- [ ] Statistics screens show zeros until there are real finished games for that user/deck/group (expected behavior, not a bug — noted here per the original scope agreement)

## Stage 4: Android Client (base)

- [x] Project initialized: Compose, Material 3, Navigation, Hilt, Room, Retrofit, kotlinx.serialization
- [x] Base theming + navigation with routes
- [x] Complete local life tracker (Room-persisted): setup, tracker, commander damage, history — 100% local, no backend
- [x] Pre-game screen: turn draw randomizer + per-player mulligan counter
- [x] Navigation flow defined (`LoginRoute → DashboardRoute → ... `), later connected to the real backend
- [x] Real auth: Credential Manager + Google Identity Services, `LoginViewModel` against `POST /auth/login`/`/auth/google`, session in `SessionManager` (DataStore), real logout
- [ ] **Improve `mapGoogleSignInError` in `LoginViewModel`** — every Credential Manager failure still collapses into the generic `LoginError.GoogleUnknown`; the `else` branch swallows the `GetCredentialException` that carries the real cause, so a misconfigured Google Cloud client is indistinguishable from a network error from the device (found 2026-09-03 while diagnosing the missing Android OAuth client; the literal it used to return became a string resource on 2026-09-04, the behaviour did not change)
- [x] Domain layer: `domain/repository/` interfaces + `*Impl`, Hilt `RepositoryModule`, `domain/usecase/` (`ResolveGameOutcomeUseCase`, `ReplayCommanderDamageUseCase`, `LoadStatisticsUseCase`) each with its own test — auth (`LoginViewModel`/`RegisterViewModel`/`SettingsViewModel`/etc.) deliberately stays outside it, still injects `AuthApi`/`SessionManager` directly
- [x] Domain owns its types: payload models moved to `domain/model/`, `data` now depends on `domain`, Room's `GameWithPlayers` mapped to `PlayedGame` — [ADR-0019](../decisions/0019-android-domain-owns-its-types.md), asserted by `ArchitectureTest`
- [x] Real repositories in `data/repository/` (decide Room vs. backend)
- [x] `CommanderApi.kt` with 15 real endpoints; separate `AuthApi.kt`
- [x] `GameState.kt`: life/turn/commander-damage/poison for 2-6 players
- [x] Full visual redesign (violet/dark palette, `AppComponents.kt`, landscape seat grid, pause overlay, game summary) + real poison counter + registration flow (`RegisterScreen`)
- [x] Internationalization: literals extracted to `strings.xml`, copy moved from Argentine "voseo" to Spain's "tuteo"; `values-en`/`values-ca` added; manual language switcher in Settings (`AppCompatDelegate.setApplicationLocales`)
- [x] Account Settings screen (username, Moxfield username, password change — gated on `has_password`)
- [x] Improve the Android life tracker — built 2026-09-03 from the Claude Design handoff (`Android Tracker.dc.html`): half-seat tap zones for life with a press flash and decorative ±, always-visible commander-damage mini grid laid out like the table (own seat marked with a person mark), expanded editor in that same layout with tap-outside-to-close, skull + pulsing red-rim elimination overlay, glowing turn ring. **Verified 2026-09-04** on a Xiaomi 13 Ultra emulator (API 37, 1440×3200, gesture navigation) against the Compose-side backend in Docker: `./gradlew lintDebug testDebugUnitTest assembleDebug` all green, plus a full playthrough (setup → pregame seat draw → tracker → pause overlay → game summary)
- [x] Edge-to-edge window (2026-09-04) — the app now paints the whole window instead of letting the system reserve and colour a strip at each end, which left a pale band under the gesture handle on a near-black screen. `enableEdgeToEdge()` with both bars transparent, safe-area insets consumed per screen (`AppScreenBackground` for the nine portrait screens, `QuadrantGrid`/`SeatGrid` for the two landscape ones), lists scrolling under the gesture bar, and full-bleed scrims for the pause/starter overlays. Prerequisite for raising `targetSdk` to 35+, where Android 15 ignores `statusBarColor`/`navigationBarColor` outright
- [x] **Every user-facing string translated** (2026-09-04) — ~30 literals were still built in Spanish inside `ApiError`, `GoogleAuthClient` and the Login/Register/Settings/JoinGame/Game ViewModels, several of them in Argentine "voseo" the rest of the app had already left behind. They now follow the `FriendsError` convention the friends screen introduced: the ViewModel names a typed error, the screen turns it into a string resource. Shared API failures go through `ApiFailure` + `ApiFailure.message()`
- [x] **The language selector actually applies the language** (2026-09-04) — `AppCompatDelegate.setApplicationLocales()` was a no-op: AppCompat needs a Context it captured itself to reach the platform `LocaleManager`, and `MainActivity` was a plain `ComponentActivity`, so the call kept the value in memory and applied nothing. `MainActivity` is now an `AppCompatActivity` (theme parent moved to `Theme.AppCompat.NoActionBar` accordingly) and `AppLocalesMetadataHolderService` is declared with the `autoStoreLocales` meta-data inside it, where AppCompat looks for it. The selected chip also reads `app_locale_tag` from the resolved resource bundle instead of a copy in the ViewModel, which is what used to show "Español" selected on an English UI
- [ ] Improve `PreGameScreen` — no detailed scope yet (single "Play" button instead of draw+start, deck thumbnails in the picker)
- [ ] **Tracker: the seat of a known player shows their commander art** — a seat assigned to a real playgroup member with a chosen deck paints its quadrant with that deck's `image_url` (Coil `AsyncImage`, `Crop`, dark scrim, seat colour kept as the border so the commander-damage grid still reads); guests, Casual mode and art-less decks keep the flat colour. Built 2026-09-06 (`GameTrackerScreen.PlayerQuadrant`, `PlayerState.deckImageUrl`, `PlayerConfigCodec`, `GameViewModel.loadJoinedDeckArt`); Android CI green on [#110](https://github.com/mllagostera/commander-companion/pull/110) — lint, unit tests and `assembleDebug`. **Still `[ ]`: never run on a device**, and a change nobody has looked at is not a visual change that works. See [DECISIONS-LOG.md](DECISIONS-LOG.md)
- [ ] **Tracker: the turn passes clockwise around the table, not down the seat list** — `nextTurn()` follows the physical quadrant ring (top row left→right, then bottom row right→left) instead of seat order, which today jumps diagonally from the top-right seat to the bottom-left one; the starter-draw spin follows the same ring, and an eliminated seat is skipped instead of being handed the turn. Built 2026-09-06 (new `SeatLayout.kt`, `GameViewModel.nextTurn`, `GameTrackerScreen`); same green Android CI run, which is where its unit tests first ran. **Still `[ ]` for the same reason as the item above**: no real table has passed a turn yet. See [DECISIONS-LOG.md](DECISIONS-LOG.md)

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
- [x] **Ping/pong heartbeat added to `internal/websocket`** (found in a 2026-08-01 review, fixed 2026-08-10): a client that drops off the network without a clean TCP close (the common case for a phone losing signal) used to leave its `Hub` entry, `readLoop`, and `writePump` goroutines running forever. `writePump` (`client.go`) now sends a ping every 54s; `readLoop` enforces a 60s read deadline refreshed on pong, closing the connection (and freeing the `Hub` entry) if none arrives. Fixing this also surfaced and fixed a pre-existing, unrelated race: `writePump` ran in a detached goroutine, so gofiber could recycle the `*Conn` back to its pool while `writePump` was still using it — `Client.startWritePump`/`wait` (a `sync.WaitGroup`) now make the connection handler block until `writePump` has actually exited before returning. Regression tests in `client_internal_test.go` open real WebSocket connections over a loopback listener (one that goes silent and must be reaped within the shrunk test deadline, one that keeps answering pings and must *not* be disconnected), run repeatedly with `-race`. See [ADR-0005](../decisions/0005-websocket-protocol.md) for the full writeup.

## Stage 7: Statistics

- [x] Real recalculation on game finish (see Stage 1 for the caveat about it being additive-only)
- [x] Aggregation queries (`UpsertUserStatistics`/`UpsertDeckStatistics`)
- [x] Real `GetUserStats`/`GetDeckStats`/`GetPlaygroupStats`
- [x] Statistics UI on Android (`StatisticsScreen`/`StatisticsViewModel`, same 3 endpoints as the web)
- [x] Head-to-head opponent stats (`GET /statistics/opponents`), per-playgroup game counts in one request (`GET /statistics/playgroups`, replaces the web client's old per-group `GetPlaygroupStats` loop), and a keyset-paginated finished-games history (`GET /statistics/games`) — all live-computed, no summary table (2026-08-05)
- [x] Web `statistics.vue`: decks/games tabs, deck sorting (recent/win rate/games played), "most played group" and "archenemy" cards from the new endpoints
- [x] Android: `FinishedGamesViewModel` + `StatisticsScreen` updated for the same finished-games history
- [x] **`GET /statistics/dashboard`** (2026-09-02): one endpoint returning exactly what the dashboard paints (totals, best deck, top 4 decks, top 3 groups, 4 recent games, streak), replacing the 30-request fan-out — same "one query instead of a client-side loop" move as `/statistics/playgroups` above. **10 queries and 4.4 KB, flat with account size**, measured on the same 400-game fixture that cost 30 requests and 539 KB before. The streak is computed in SQL (`GetCurrentStreakForUser`) rather than by shipping every game to the client. 7 integration tests; `openapi.yaml` updated. See [DECISIONS-LOG.md](DECISIONS-LOG.md)

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

- [x] Friends system: send/accept/reject requests, friends list — distinct from `playgroups` (game groups, not friendship relations) (**all three phases done**, 2026-08-15/17, see [ADR-0017](../decisions/0017-friends-system.md) including its 2026-08-17 amendments): backend (`internal/friends`, migration `00017_friend_requests.sql`) + web (`friends/index.vue`, add by username search; QR in `settings.vue`) + Android (`FriendsRepository`, friends screen with search/accept/reject/cancel/remove, QR generation in Settings, and scanning via Play Services' code scanner — **no `CAMERA` permission**). The QR encodes a deep link (`/friends/add/{id}`) rather than a bare UUID, so any camera app resolves it: with the app installed the App Link opens it there, without it the browser lands on `web/app/pages/friends/add/[id].vue`, which sends the same request. Both clients still accept a bare UUID, so older codes scan. Web has no camera scanning by design (see ADR-0017). **Still open:** App Links *verification* — `/.well-known/assetlinks.json` needs the release keystore's SHA-256 fingerprint, which isn't in the repo; until it's added the link opens the web page instead of the app directly. Android release builds also need `-PWEB_APP_URL=https://<dominio>`, or the generated QR points at the emulator default.
- [ ] Groups beyond the existing `playgroups`, or extend them — not needed for tournaments in the end (see below), still open for any future friends/groups work
- [x] **Standalone Swiss-format Commander tournaments** (2026-08-09, pulled forward ahead of the rest of this stage — see [ADR-0016](../decisions/0016-swiss-tournament-format.md) for the full design and the alternatives ruled out): `backend/internal/tournaments` (migration `00016_tournaments.sql`, five new tables, none touching `games`/`game_players`), organizer creates a tournament and gets a join code, app users self-register with one of their own decks (`POST /tournaments/join`), the organizer adds guests with no account (`POST /tournaments/{id}/participants`), `POST /tournaments/{id}/start` locks the roster (valid 3-4 player table split required, min 3, max 4 per table) and seats round 1 via a greedy repeat-avoiding pairing heuristic (`pairing.go`), round count is computed automatically from the roster size, the organizer records each table's finish order (`POST /tournaments/{id}/tables/{tableId}/result`, scoring 2/1/0/0) and advances rounds (`POST /tournaments/{id}/rounds/next`) until the tournament finishes, and `GET /tournaments/lookup?code=` is the "enter the code" read-only lookup of a participant's current table. Deliberately **not** wired into the live `games`/`game_players`/WebSocket life tracker or into Android — both explicitly out of scope for this pass, see the ADR. Full test coverage (`pairing_internal_test.go` for the pairing math, `service_test.go` including a full 3-round tournament lifecycle test against real Postgres) and web UI (`web/app/pages/tournaments/`).

- [x] **Deleting a tournament created by mistake** (2026-09-02, see [DECISIONS-LOG](DECISIONS-LOG.md)): `DELETE /tournaments/{id}`, organizer-only and only while `status = 'registration'` — the one state where nothing has been played yet and the tournament is still purely the organizer's; once it starts, its rounds and results belong to its participants too (409 `ErrTournamentNotDeletable`; not-yours is a 404, same "don't reveal" criteria as the rest of the module). Deletes the roster and the tournament in one transaction (`00016_tournaments.sql` has no `ON DELETE CASCADE`); rounds/tables/seats need no cleanup precisely because of the registration gate. Web: organizer-only delete button in the registration phase of `tournaments/[id].vue` behind the same `useModalA11y` confirm dialog `admin/users/[id].vue` uses, i18n en/es/ca. Three service tests cover the happy path, the non-organizer 404 and the already-started 409.

## Stage 10: Admin dashboard

- [x] **Phase 1: role foundation + user management + global stats overview** (2026-08-17, see [ADR-0018](../decisions/0018-admin-role-and-user-moderation.md)): `users.is_admin`/`users.is_active` (migration `00018_admin_and_account_status.sql`), `auth.RequireAdmin` middleware (checks `is_admin` fresh from the DB per request, not a JWT claim), new `backend/internal/admin` module (`GET /admin/users` paginated+searchable, `GET /admin/users/{id}` with deck/games-played counts, `PATCH /admin/users/{id}/status` activate/deactivate with a self-lockout guard, `GET /admin/stats/overview`), `is_active` gating added to login/Google-login/refresh (`ErrAccountDeactivated`). Web: `/admin`, `/admin/users`, `/admin/users/{id}` pages (Nuxt), `admin.ts` route guard, `useAdmin.ts` composable, nav entry shown only to admins, i18n (en/es/ca). Verified end-to-end in a browser (promote a user by hand in the DB, log in, deactivate/reactivate, confirm a non-admin is redirected and a deactivated account gets 403 on login) — see DECISIONS-LOG.md.
- [x] **Same-day addendum: live "online users"/"active games" card + historical activity chart** (2026-08-17, see ADR-0018's Addendum): `GetAdminOverviewStats` gained `online_users` (distinct users with an unexpired/unrevoked refresh token — a session proxy, not real presence) and `active_games` (`games.status = 'active'`, direct count); new `GET /admin/stats/activity?days=` (default 30, clamped `[1,90]`) derives a daily `games_started`/`active_users` series from `games`/`game_players` (no snapshot table, no scheduler), gaps filled to zero by `admin.Service.GetDailyActivity`. Web: `AdminActivityChart.vue` (hand-rolled SVG line chart, hover crosshair+tooltip, sr-only accessible data table, palette validated with the dataviz skill's `validate_palette.js`, new `--chart-series-1`/`--chart-series-2` tokens in `main.css`), a "Live" card on `/admin`. Verified end-to-end in a browser in both themes (hover tooltip, legend, gridlines, accessible table row count).
- [ ] Phase 2+ (not started): moderation of games/tournaments/decks, an admin-promotion UI (today `is_admin` is only set by hand in the DB), an audit log of admin actions, full session/token revocation on deactivation (currently bounded to one access-token TTL, see ADR-0018).

## Cross-cutting (quality, infrastructure, security)

- [x] Architecture guardrails: `depguard` (backend), Konsist (Android), `eslint-plugin-boundaries` (web) and `.github/scripts/check-architecture.sh` + `architecture-ci.yml` for what no import-level tool can express — see AGENTS.md §7
- [ ] **Add `architecture invariants hold` to the required checks on `main`** — external manual step in GitHub's branch protection settings; until then `architecture-ci.yml` reports but does not block (the other three guardrails ride gates that are already required)
- [x] Backend integration tests (real Postgres) across all 9 modules with queries
- [x] Android unit tests (JUnit + `kotlinx-coroutines-test`, hand-written fakes, no mocking framework)
- [x] CI/CD: `backend-ci.yml`, `android-ci.yml`, `web-ci.yml`, `docs-ci.yml` — all four follow the same always-reports-a-check `changes`/`dorny-paths-filter` pattern
- [x] **Every CI check name in English** (2026-09-02): 8 job/step names were still in Spanish. Renaming them moves the identifiers branch protection matches on, so the 4 required contexts affected were updated in the same pass
- [x] **i18n checks for web and Android** (2026-09-02): `.github/scripts/check-i18n-{web,android}.mjs`. A missing key fails nothing else — Vue renders the key with a 200, Android falls back to the default locale — which is how #101 shipped. Both verify locale parity and that every key used in the source resolves; Android honours `tools:ignore="MissingTranslation"`. Both mutation-tested. Not in the required-checks list yet
- [ ] **The two production deploys race each other**: Render and Vercel each deploy off their own GitHub integration on a push to `main`, in parallel and unordered, so the new frontend can serve against the old backend for as long as Render takes to build (see [ADR-0015](../decisions/0015-deployment-infrastructure.md)). **No longer blocked**: since 2026-09-06 `/health` reports the `commit` it was built from, so "wait for the backend" can be resolved by polling it (see [ADR-0020](../decisions/0020-build-provenance-in-health.md)). What is still open is the sequencing itself — who polls, and how Vercel is held back
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
- [x] Deployment infrastructure stack decided — [ADR-0015](../decisions/0015-deployment-infrastructure.md) (Render + Vercel + Supabase); `render.yaml`/a deploying CI workflow remain a future improvement, deployment is manual today

---

## Suggested order of work

The original 9-step ordering (auth → database → statistics → OpenAPI → Android →
WebSocket → Moxfield) is **complete**; each of its steps survives as `[x]` items
in its own stage above, so nothing is lost by not repeating it here. It was also
carrying three caveats that had stopped being true: `FinishGame`'s
double-counting (fixed 2026-08-01), the missing WebSocket heartbeat (fixed
2026-08-10), and the bulk Moxfield import being blocked on confirming the real
endpoint (confirmed end to end 2026-08-03).

What is left is no longer a sequence of build-outs. It splits three ways:

**Blocked on you, not on code** — nothing here can be finished by writing Go or
Kotlin, and two of them are what still keeps the app in alpha:

1. **Google Cloud OAuth credentials** (Web + Android Client ID) — until they
   exist, Google Sign-In cannot be tested against real Google on either client
   (Stage 1). Both sides are implemented and waiting on the value.
2. **A verified domain on Resend** (SPF/DKIM/DMARC) + a real dashboard template,
   to turn on `REQUIRE_EMAIL_VERIFICATION=true` and leave alpha (Stage 1).
3. **Add `architecture invariants hold` to the required checks on `main`** —
   a change in GitHub's branch protection settings. Until it is made, the
   guardrails added on 2026-09-03 report but do not block (Cross-cutting).

**Worth fixing next, in this order** — real problems with a known shape:

4. **The Render/Vercel deploy race** (Cross-cutting) — the two production
   deploys run in parallel and unordered on a push to `main`, so the new
   frontend can serve against the old backend. This is the only open item that
   can produce a broken production, which is why it leads. The constraint that
   used to sit here is gone: since 2026-09-06 `/health` reports the `commit` it
   was built from ([ADR-0020](../decisions/0020-build-provenance-in-health.md)),
   so polling it now answers "is the new backend live?". What is left is
   deciding who polls and how Vercel is held back until it answers.
5. **The deferred WCAG 2.2 AAA findings** (Stage 4b) — the audit's technical
   fixes all landed; what remains needs a **design decision**, not code: the
   contrast of `--text-dim`/`--accent-link`/`--text-muted`, and the tracker's
   fixed black text over arbitrary per-player colors. Decide the palette, and
   the implementation is small.
6. **Admin dashboard Phase 2+** (Stage 10) — games/tournaments/decks moderation,
   an admin-promotion UI (`is_admin` is set by hand in the DB today), an audit
   log, and full session revocation on deactivation (bounded to one access-token
   TTL today, see [ADR-0018](../decisions/0018-admin-role-and-user-moderation.md)).

**Needs scoping before it can be ordered** — these are currently one-line wishes,
and the first useful step on each is deciding what it actually means:

7. The `PreGameScreen` pass (Stage 4). The two tracker items next to it are
   no longer wishes: the commander-art seat background and the clockwise turn
   order were scoped on 2026-09-06 (design in DECISIONS-LOG.md) and can be
   picked up as they are.
8. The web client's visual polish pass, and whether the dashboard wants
   shell-first rendering + skeletons now that the endpoint has landed (Stage 4b).

Ongoing, not steps: record an ADR for each non-trivial decision, and keep tests
and CI current with the module being built rather than leaving them for the end.
