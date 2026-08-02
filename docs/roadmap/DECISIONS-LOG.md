# Commander Companion — Decisions & History Log

Companion to [TASKS.md](TASKS.md). TASKS.md is a compact status checklist —
what's done, what's not, one line each. This file is the narrative behind
it: what changed, why, gotchas hit along the way, how it was verified,
dates, PRs, and the exact user requests that triggered each pass. It exists
because that narrative is genuinely useful project history (TASKS.md's own
rules say not to delete it) but loading all of it on every new session was
the single biggest avoidable token cost in this repo — as of 2026-08-01,
TASKS.md's "Last reviewed" field alone was a single ~30,000-character line.

This file is split in two:
- **Audit / session history** — the chronological log of review passes, in
  the reverse-chronological "newest first" convention the project already
  used, condensed from what used to be TASKS.md's "Last reviewed" field.
- **Per-stage narrative** — organized under the same Stage headers as
  TASKS.md, one entry per non-trivial item, for the detail that didn't fit
  in a single status line.

Not every TASKS.md item has an entry here — short, self-explanatory items
don't need one. When TASKS.md links here for an item, the corresponding
Stage section below has the detail.

---

## Audit / session history (newest first)

**2026-08-01 — Documentation restructuring.** Explicit user request, after a
full project review that also flagged two code issues (see the Stage 1
entries on `payload.amount` validation and the `FinishGame` double-counting
race). Split TASKS.md's "Last reviewed" field — a single ~30KB line
chaining "Previous review, same day: ..." entries going back to
2026-07-26 — into this file. TASKS.md keeps one line per item; this file
keeps the narrative. Nothing was deleted, only moved and, where it was
pure repetition across three or four passes over the same paragraph,
trimmed.

**2026-08-01 — Security audit follow-up #3** ("los headers de seguridad del
cliente web, dale"): closed the last open Low finding from the same-day
audit — the web client sent no `Content-Security-Policy`/
`Strict-Transport-Security`/`X-Frame-Options`/`X-Content-Type-Options`/
`Referrer-Policy`. New `web/server/plugins/security-headers.ts` +
`web/server/utils/security-headers.ts` set all five on every response,
gated to non-dev only (`!import.meta.dev` — Vite's dev client needs
eval-based HMR and a same-origin WebSocket this CSP would block, and dev is
never internet-facing). The one real design decision: `script-src` isn't
`'unsafe-inline'` (that would defeat the point of adding a CSP at all) —
Nuxt always renders at least one inline `<script>` (the
`window.__NUXT__=...` runtime-config payload, sometimes a Vite
`type="importmap"` one too), so a first attempt at nonce-based CSP via the
`render:html` hook was tried and empirically broke hydration (the
importmap script isn't reachable through that hook, confirmed via real
console CSP-violation errors). Switched to hashing every inline script's
actual content from the final composed HTML body (`beforeResponse` hook)
and allowlisting those exact hashes — verified end-to-end with a headless
Chromium session against a real `npm run build` + `node
.output/server/index.mjs`: no CSP console errors, real hydration (typed
into the login form, read the value back), working SPA navigation.
`style-src` keeps `'unsafe-inline'` (the app uses `:style="..."` bindings
in several places, CSP has no hash/nonce mechanism for those) — documented
as a deliberate scope limit in `web/README.md`. This closed the audit
entirely except two Low items left open on purpose (rate limiter behind an
unconfigured `ProxyHeader`, the `cc_session` marker-cookie tradeoff — both
already documented in-repo as accepted).

**2026-08-01 — Security audit follow-up #2** ("acabemos con el refresco e
[sic] token por robo"): closed the last open Medium finding — refresh
token rotation (`internal/auth/service.go: Refresh`, see
[ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)) had no
theft/reuse detection. Reusing an already-revoked refresh token (rotated
away by an earlier `Refresh`, or killed by `Logout`) is now treated as a
signal of theft: `Refresh` revokes *every* active refresh token for that
user (`RevokeAllRefreshTokensForUser`) instead of just rejecting the reused
one, so a stolen token replayed after the legitimate client already
rotated past it can't keep regenerating a session indefinitely — the
account owner is logged out everywhere and has to log in again, same as
standard OAuth2 refresh-rotation. New regression test
`TestRefresh_ReuseOfRotatedToken_RevokesTheWholeSessionFamily`.

**2026-08-01 — Security audit follow-up #1** ("avanza con el 1, 2 y 4" over
the Medium findings list): closed 3 gaps. (1) `statistics.GetPlaygroupStats`
had no membership check — any authenticated user with a playgroup UUID
could see its aggregated stats; `statistics.Service` now takes a
`PlaygroupMembership` dependency, denies with `ErrPlaygroupNotFound`
(no-reveal) if the caller isn't a member. (2) `games.CreateGame` never
validated the caller belonged to `playgroup_id` — any user could create
games "belonging" to a group they had no relation to; now requires
membership too. (3) `GET /users/search` had no rate limit of its own —
added `newSearchRateLimiter` (20 req/min/IP, `cmd/api/main.go`);
`openapi.yaml` gained the `429` response. New regression tests:
`TestGetPlaygroupStats_NotAMember_ReturnsNotFound`,
`TestCreateGame_NotAMemberOfPlaygroup_ReturnsNotFound`. The `/auth/register`
409 enumeration flagged in the same audit was left as-is (already covered
by the auth rate limit; revealing "email taken" is normal registration UX).

**2026-08-01 — Security audit + fixes** ("soluciona los críticos y altos"):
closed two Critical, pre-existing authorization gaps, unrelated to any
specific feature request. `GetGame`/`StartGame`/`FinishGame`/`GetTimeline`
never checked the caller had any relationship to the game — any
authenticated user could read, forcibly start, or forcibly finish ANY game
in the system, corrupting its statistics and broadcasting a fake
`game_finished` event to real participants; and the global `GET /games`
(no `playgroup_id`) returned every game in the database with no per-user
filter. Fixed: `internal/games/service.go` gained `authorizeGameAccess`
(playgroup membership if the game has one, otherwise a seat in it, same
trust boundary as ADR-0013's proxy-join), `ListGamesPage` now filters by
`user_id` via `EXISTS (... game_players ...)`, and the WebSocket handler
gained a `MembershipChecker` (`games.Service.CanAccessGame`) so a
connection is rejected before being registered in the Hub if the caller
can't access that game. New regression tests in
`internal/games/service_test.go`. Also fixed, same pass: the web BFF's
`/api/backend/[...path]` blocked `auth/*` with a case-sensitive
`startsWith`, bypassable via `/api/backend/Auth/login` since Fiber's router
is case-insensitive by default — now lowercases the path first. And the
backend now fails fast at boot (`internal/config/config.go`) instead of
silently falling back to the publicly-known default `JWT_SECRET` or an
open `CORS_ALLOWED_ORIGINS=*` when `APP_ENV=production` and those aren't
explicitly set — previously a forgotten env var in a real deploy meant a
total auth bypass with a secret visible in this repo's own git history.
The remaining Medium/Low findings (ownership check on `CreateGame`, no
refresh-token-reuse detection, missing security headers) were intentionally
left open for that pass — see the three follow-ups above, all since closed.

**2026-08-01 — Android: manual language switcher, domain layer, WebSocket
client, live reconciliation, statistics UI, second-device join.** Several
same-day passes, all explicit user requests, all landed without a
`./gradlew` run in this environment (the sandbox's network policy blocks
`dl.google.com`, so the Android Gradle Plugin can't be resolved here — each
was reviewed by hand against existing patterns instead; each needs a real
`assembleDebug lintDebug testDebugUnitTest` run in an environment with
Google Maven access before merging). Full detail under Stage 4/5/6/7 below.
Also this day: `web/vercel.json` gained an `ignoreCommand` so an
Android-only PR doesn't trigger a no-op Vercel preview deploy (see
[ADR-0015](../decisions/0015-infraestructura-de-despliegue.md)); and
housekeeping — confirmed `.github/modernize/java-upgrade` as scaffolding
unrelated to this project's stack (removed where present) and trimmed
duplicated "4 sources of truth"/layer-pattern narrative that
`README.md`/`ARCHITECTURE.md`/`ROADMAP.md` had all been independently
restating (`ARCHITECTURE.md` is now the single canonical location).

**2026-08-01 — Real fullscreen for the web life tracker** (user request):
`app/pages/play.vue` already filled the browser window via CSS but never
requested real browser fullscreen. New `useFullscreen.ts`
(`isFullscreen`/`isSupported`, `toggleFullscreen()` via
`requestFullscreen()`/`exitFullscreen()`, synced with the native
`fullscreenchange` event) and a toggle button next to the existing `✕`.

**2026-07-30 — `main` sync after merging PRs #48/#49/#50.** Audited 3 real
changes against TASKS.md, which mentioned none of them yet.
- **Backend (#48)**: the `api` binary now applies embedded goose migrations
  on startup (`internal/common/migrate.go`), because Render's free tier
  offers no "pre-deploy/release command" hook — migrations were never
  actually being applied in production against Supabase before this.
  `backend/Dockerfile` now copies `migrations/` into the final image.
  `backend/README.md` gained a "Deployment (Render)" section (env vars,
  Supabase Session pooler — not Transaction pooler, the backend uses
  prepared statements via pgx) and a documented limitation: with >1
  replica, all of them would run `goose up` in parallel on boot (goose is
  idempotent but the runs aren't serialized with a lock) — harmless today,
  single-instance deployment.
- **Web (#49)**: full internationalization with `@nuxtjs/i18n` — see
  [ADR-0014](../decisions/0014-internacionalizacion-web.md) (single active
  locale `es`, `strategy: 'no_prefix'`, ~200-250 keys, and along the way
  fixes the copy from Argentine "voseo" to Spain's "tuteo"). Same PR: the
  Google Sign-In button moves to `pill` shape with a theme-dependent style;
  Nuxt gained a default `<title>`/`titleTemplate` (previously none — a
  WCAG 2.4.2 violation flagged by axe/Lighthouse).
- **Android (#50)**: full visual redesign (violet/dark palette, new
  `Gradients.kt`, new shared `AppComponents.kt` replacing `PlayerCard.kt`).
  `PreGameScreen`/`GameTrackerScreen` now require landscape orientation
  with a new seat grid, pause overlay, and final game summary — none
  existed before. **Functional, not just visual**: the poison counter goes
  from unmodeled to real (`PlayerState.poison`,
  `PlayerState.isEliminated()` now centralizes all 3 elimination
  conditions, previously only 2 lived in a private `isAlive`). New
  registration flow (`RegisterScreen`/`RegisterViewModel`) — didn't exist
  on Android before, only on web.

**2026-07-28 — Documentation audit** ("check whether we've left any
documentation out of date"): 3 real gaps found, all for code already
committed earlier the same day but never reflected in TASKS.md — the
Playgroup screens on the web client, the Settings screen (password change
+ Moxfield linking + bulk import trigger), and a stale description of
`JoinGame` that predated the proxy-join work (ADR-0013). `openapi.yaml` and
the ADRs were already current; the gap was specific to TASKS.md.

**2026-07-28 — `openapi.yaml` audit against real routes**: ~39 routes
across 10 modules enumerated and compared one by one against the spec. 3
real discrepancies fixed: `GET /games/{id}` was undocumented (added);
`GET /users/{id}` was documented but has no handler (removed from spec);
`PATCH /decks/{id}` was documented but doesn't exist (removed). The
remaining ~36 paths verified field-by-field against their Go DTOs with no
further divergence found.

**2026-07-28 — Search users, proxy-join, Casual/Group mode.** Same day, 3
related passes. `GET /users/search?q=` closed a literal TODO in the web's
"add member" form ("ask your friend for their user ID"); found and fixed a
real bug along the way — no results returned JSON `null` instead of `[]`,
breaking any JS/TS client that assumes an array. Backend gained
`game_players.added_by` + proxy-join (`POST /games/{id}/join` with
`user_id`, gated by shared playgroup membership) — see
[ADR-0013](../decisions/0013-proxy-join-y-autorizacion-de-acciones.md) —
and closed, in the same change, a pre-existing gap where
`POST /games/{id}/actions` never validated that `actor_id` belonged to the
caller at all. Android's `PlayerSetupScreen` gained a Casual/Group
selector on top of that: Group mode lets you pick a real playgroup and
assign members (with their decks) to seats, seating them via proxy-join in
one bootstrap call; this superseded an explicit "that's me" chip + own-deck
picker that had been added hours earlier the same day (kept in the log
below as a superseded step, not deleted).

**2026-07-27 — Cross-cutting infra cleanup**: confirmed `docker/`/`scripts/`
never existed (no git history for them, tree diagram fixed); added
`.github/dependabot.yml` (4 ecosystems, weekly); found and fixed a real CI
gap — `backend-ci.yml`/`android-ci.yml`/`web-ci.yml`'s path filters also
matched each directory's own README, so a README-only change triggered the
full heavy battery (for backend: real Postgres + migrations + lint). Fixed
with `predicate-quantifier: 'every'` + `!*/README.md` exclusions.

**2026-07-27 — Full documentation audit**, centralizing the root
`README.md` as the index of every doc in the repo (previously linked only
3). Main finding: the Android UX/navigation docs had been written in a
parallel worktree at the same commit that connected Android to the real
backend, and were left describing the pre-connection app. All corrected.
Also corrected: several ADRs that still described superseded states
(ADR-0004 said Moxfield/statistics were pending when they were done;
ADR-0009 said "no repository layer" when one existed; ADR-0010 said
`internal/websocket/` was empty when it wasn't), a stale README claim that
`web-ci.yml` didn't exist, and several TASKS.md items that were already
done but still unchecked.

**2026-07-27 — PostgreSQL 18, deck image, docker-compose consolidation,
Stage 0 closed, initial Android↔backend connection.** PostgreSQL pinned to
18 everywhere (previously misaligned 15/16 between compose and CI; the
volume mount layout also changed in 18+, `/var/lib/postgresql` instead of
`/var/lib/postgresql/data`). Imported decks now carry `image_url` (art crop
from Moxfield's own `main.id` field, migration `00006`). `docker-compose.yml`
centralized at the repo root with a real `web/Dockerfile`, fixing two bugs
found while bringing up the full stack: `api` not waiting for Postgres to
be *healthy*, and SSR calls from the `web` container unable to resolve
`localhost:8080` (split `NUXT_API_BASE` internal vs. `NUXT_PUBLIC_API_BASE`
browser-facing). `backend/README.md` created. Stage 0's 4 pending
deliverables (use cases, wireframes, diagrams, foundational ADRs) closed
the same day. Android connected to the real auth backend for the first
time (`LoginViewModel`, `NetworkModule`, `SessionManager`,
`AuthInterceptor`/`AuthAuthenticator`) — only the external Google Cloud Web
Client ID step remained. Stage 2 (database) closed: explicit indexes,
CHECK constraints, exported ER diagram. Initial Nuxt web client skeleton
scaffolded (email/password + Google login). Stage 6 WebSocket: protocol
design ([ADR-0005](../decisions/0005-websocket-protocol.md)) and real
server implementation (`internal/websocket/`) both landed.

**2026-07-26 — Initial audit and backend build-out**: auth (Google OAuth
included), generated the `playgroups`/`games`/`game-actions` slices,
sqlc/lint tooling fixes, GitHub Actions quality gates, repo linked with
branch protection on `main`, real email/password + Google auth, CORS, the
manual `tools/auth-test/` tool, real `decks`/Moxfield wiring, first backend
integration tests, real `games`/`game-actions` (the game engine itself),
real `playgroups`/`statistics` wiring (recalculation on game finish) — by
the end of this day, **the backend was 100% connected to the real
database, no dummy modules left**. Also: the complete Android local life
tracker with Room persistence, and `PreGameScreen` with the turn
randomizer and mulligan tracking.

---

## Per-stage narrative

### Stage 1 — `payload.amount` validation (found and fixed 2026-08-01)

`internal/game-actions/service.go`'s `payloadAmount` only checked that
`payload.amount` existed and was numeric — no sign check, no range check.
Reproduced against a real Postgres instance before fixing:

- `CombatDamage` with `amount: -500` was accepted and *healed* the target
  (life went up, and the negative amount was added to
  `total_damage_dealt`).
- `CombatDamage` with `amount: 3e9` was accepted: `int32(float64(3e9))`
  doesn't error, it silently wraps to a large *negative* int32 — the
  target's `life_total` became `-2147483608` (instant elimination, but for
  the wrong reason and with a nonsensical stored value).
- `PoisonCounter` with `amount: -50` was accepted, producing
  `poison_counters = -50`.
- `CommanderDamage` with a negative amount was the one case already caught
  — by the database's own `CHECK (amount >= 0)` on `commander_damage`,
  surfacing as an ugly `500` instead of a clean `400`.

Fixed: `CombatDamage`/`CommanderDamage` now go through
`payloadPositiveAmount` (rejects `<= 0`) since their `amount` is a
quantity of damage, never a signed delta; `LifeChange`/`PoisonCounter` keep
accepting negative values on purpose (a signed delta, e.g. to correct a
misclick) but every `payload.amount` — regardless of action type — is now
bounded to a whole number with magnitude ≤ 1,000,000
(`maxAmountMagnitude`, `payloadAmount`), rejecting fractional values,
`NaN`, and anything that would risk wrapping past `int32`'s range. New
errors: `ErrAmountOutOfRange`, `ErrAmountMustBePositive`. Regression tests
added in `internal/game-actions/service_test.go`:
`TestRecordAction_DamageAmount_MustBePositive`,
`TestRecordAction_AmountOutOfRange_ReturnsBadRequest`,
`TestRecordAction_NonIntegerAmount_ReturnsBadRequest`,
`TestRecordAction_PoisonCounter_NegativeAmountStillAllowed` (locks in that
poison correction still works, since it's deliberately not restricted the
same way as damage). `openapi.yaml`'s `/games/{id}/actions` payload
description and 400 response updated to match. Full backend test suite
(`go test -race -p 1 ./...`) and `golangci-lint` verified clean after the
change.

### Stage 1 — `FinishGame` concurrency / statistics double-counting (found and fixed 2026-08-01)

Two issues that compound:

1. `games/service.go: FinishGame` reads `game.Status != statusActive`
   outside any transaction, several round-trips before the actual
   `UPDATE`. The SQL itself
   (`internal/games/query.sql: FinishGame`) has no `AND status = 'active'`
   guard — it's `UPDATE games SET status = 'finished', finished_at = now()
   WHERE id = $1`, full stop. Two concurrent `FinishGame` calls on the same
   game can both pass the check and both succeed.
2. `statistics.RecalculateForGame` is named like a recompute, but it's
   purely additive (`ON CONFLICT DO UPDATE SET games_played =
   user_statistics_summary.games_played + EXCLUDED.games_played`, same
   pattern for every other counter). There's no path to recompute a user's
   or deck's stats from scratch — once inflated by a duplicate run, it
   stays inflated.

Reproduced with a Go test issuing 8 concurrent `FinishGame` calls (from
both seated players' callers) against one real 2-player game over
Postgres: all 8 succeeded, and both players' `games_played`/derived
counters ended up multiplied by 8x. With 2 concurrent calls it's
racier (often only 1 succeeds), but it does happen. The realistic trigger
isn't exotic: two players tapping "Finish" at the same time in Group mode,
or a client retrying after a timeout.

This directly contradicts `schema.dbml`'s own note on
`user_statistics_summary`/`deck_statistics_summary`: "Pre-calculated
statistics (recalculated if a historical game is modified)" — that's not
actually possible today. See [ADR-0011](../decisions/0011-estrategia-migraciones-y-recalculo-estadisticas.md),
which already flags "`RecalculateForGame` is incremental, not idempotent —
each game must be processed exactly once" as an invariant to respect, but
nothing currently enforces it.

**Fixed the same day.** `internal/games/query.sql`'s `FinishGame`/`StartGame`
queries now guard their `UPDATE` with the status they require
(`AND status = 'active'`/`'pending'`) instead of relying solely on the
service's earlier read-then-write check — that check is kept as a
fast-path (avoids the write entirely for an obviously-wrong-state game),
but the guard on the `UPDATE` itself is what actually makes the state
transition atomic. `internal/games/service.go` now treats `pgx.ErrNoRows`
from either call as the corresponding existing domain error
(`ErrGameNotActive`/`ErrGameAlreadyStarted`, both already mapped to `409`)
instead of wrapping it as an unexpected error. `sqlc generate` re-run,
diff reviewed (only the two queries' `WHERE` clauses changed, nothing
else). New regression test,
`TestFinishGame_Concurrent_OnlyOneSucceedsAndStatsAreNotDoubleCounted`
(`internal/games/service_test.go`): fires 8 concurrent `FinishGame` calls
(alternating between both seated players) against one real Postgres-backed
game, asserts exactly one succeeds and the rest get `409`, and asserts
both players' recalculated `games_played` is exactly 1 — run 5x with
`-race` with no failures. Full backend suite (`go test -race -p 1 ./...`,
16 packages) and `golangci-lint` verified clean after the change.

This closes the immediate bug (double-counting under a race) but doesn't
add a general recompute-from-scratch capability: `RecalculateForGame` is
still purely additive by design. That's now provably safe, since the guard
above guarantees it's only ever invoked once per game's finish transition
— but it still means there's no way to *re-derive* `user_statistics_summary`/
`deck_statistics_summary` if the aggregation formula itself ever changes,
which is exactly the gap [ADR-0011](../decisions/0011-estrategia-migraciones-y-recalculo-estadisticas.md)
already flagged and proposed a (never-implemented) `recalculate-stats`
command for. Left as a separate, larger piece of future work.

### Stage 4/5 — `SessionManager.refreshAccessToken()` concurrency (found and fixed 2026-08-01)

`AuthAuthenticator.authenticate()` calls `runBlocking { sessionManager.refreshAccessToken() }`
synchronously, on whichever OkHttp dispatcher thread received the 401 — and
OkHttp genuinely runs concurrent requests on separate threads, so this is a
real multi-threaded call site, not just a theoretical one. The scenario
that makes it likely rather than exotic: `LoadStatisticsUseCase` fires a
batch of `async`/`awaitAll` calls (global stats, then every deck's and
every playgroup's stats) that all share the same access token; once that
token expires, opening the Statistics screen can 401 several requests at
once. Before this fix, `refreshAccessToken()` had no synchronization at
all — each concurrent caller independently read the stored refresh token,
called `POST /auth/refresh`, and saved the response. Since the backend
**rotates** the refresh token on every use (see
[ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)) and, as
of the 2026-08-01 backend security-audit pass above, treats presenting an
already-rotated-away refresh token as a signal of theft — revoking the
*entire* session family, logging the user out on every device — two
concurrent refresh calls racing against each other could trigger exactly
that: the loser presents a token the winner already rotated away,
`SessionManager.forceLogout()` fires, and the user is logged out
everywhere over what was really just a timing coincidence between two of
their own requests.

**Fixed the same day** with a new `core/util/SingleFlight.kt`: a small,
Android/Context-free class that collapses concurrent callers of a suspend
block into a single real execution, sharing the one result — the same
shape as the web client's own `inFlightRefresh` map
(`web/server/utils/backend.ts`), which already solved the identical
problem in the Nitro BFF (see that file's own doc comment, which
independently arrived at the same fix for the same underlying reason).
`SessionManager` gained a class-level `refreshScope`
(`CoroutineScope(SupervisorJob() + Dispatchers.IO)`, deliberately NOT tied
to any individual `runBlocking` call's own scope — it needs to outlive
every one of them for the dedup to work across separate OkHttp threads)
and now runs `refreshAccessToken()`'s body through
`refreshSingleFlight.run { ... }`.

`SessionManager` itself remains untestable in a pure-JVM test (the
pre-existing, documented gap: it has a `Context` constructor parameter, no
Robolectric in this project) so the fix couldn't get a `SessionManagerTest`
the normal way — same reasoning already on file for
`SettingsViewModel`/`DashboardViewModel`/etc. Followed the project's own
established workaround instead (see `GameSocketReconnect.kt`'s doc: "kept
as a plain function... so the retry/backoff behavior can be unit tested...
without a real socket"): `SingleFlight` is a standalone, dependency-free
class with its own test, `SingleFlightTest.kt` — 3 cases: N concurrent
callers collapse into exactly one execution of the block and all receive
its result; a call made after the previous one completed triggers a fresh
execution (not permanently cached); and a failing block's exception
propagates to the caller while still clearing the in-flight state so the
next call retries instead of getting stuck.

**Verification note**: this sandbox's network policy still blocks
`dl.google.com`, so `./gradlew` couldn't run here, same limitation as
every other Android pass in this log. Unlike those previous passes, this
one wasn't only reviewed by hand — the risk was concentrated entirely in
`SingleFlight.kt`'s coroutine logic, which has zero Android/Hilt/Room
dependencies, so it was possible to fetch a standalone Kotlin compiler
(2.2.10, matching `libs.versions.toml`) and `kotlinx-coroutines-core`/
`-test` straight from Maven Central (reachable even though
`dl.google.com` isn't), compile `SingleFlight.kt` + `SingleFlightTest.kt`
outside Gradle entirely, and run the tests with real JUnit — all 3 passed.
The exact `return@run` early-return idiom used in `SessionManager`'s new
`refreshAccessToken()` body (`someSingleFlightInstance.run { ... return@run
null ... }`) was independently compiled and executed in isolation too, to
confirm Kotlin resolves the label to `SingleFlight.run`'s lambda and not
something else. Still needs a real `assembleDebug lintDebug
testDebugUnitTest` run in an environment with Google Maven access before
merging, same as every other Android change in this log — this only
closes the gap for the specific new logic, not for the full app.

### Stage 2 — `schema.dbml` drift from real migrations (found 2026-08-01, not yet fixed)

Verified by compiling `schema.dbml` to SQL (`dbml2sql`), loading it into a
throwaway Postgres database, and diffing `information_schema.columns`
against a second database with the real goose migrations applied. Result:
the entire `deck_resync_jobs` table (added by migration
`00013_deck_resync_jobs.sql`, part of Stage 8's "resync all decks" feature)
is missing from `schema.dbml`. No other drift found — the rest of the
schema matches exactly, and `docs/api/openapi.yaml` is independently
correct against the real registered routes (verified separately).

This matters beyond one missing table: `docs-ci.yml`'s `dbml-validate` job
only checks that `schema.dbml` compiles to valid SQL, not that it matches
the actual database — so a migration can land without its DBML counterpart
and CI stays green indefinitely, in direct contradiction of README.md §3's
rule to "edit `schema.dbml` first." **Not fixed in this pass** (out of
scope for the documentation-restructuring session it was found in) — left
as an open TASKS.md item: add the missing table, and ideally add a CI
check that applies migrations + compiles the DBML + diffs
`information_schema` so this class of drift gets caught automatically.

### Stage 6 — No WebSocket heartbeat (found 2026-08-01, not yet fixed)

`internal/websocket/handler.go` sets a read deadline only for the initial
auth handshake (`authMessageTimeout`, 10s), then clears it entirely
(`SetReadDeadline(time.Time{})`) once authenticated. `client.go`'s
`readLoop` is then an unbounded `ReadMessage()` loop with no ping/pong
anywhere in the package. A client that disappears from the network without
a clean TCP close — the ordinary way a phone loses connectivity, not an
edge case for this app's primary use case (a life tracker meant to sit on
a table during a game) — leaves its `Hub` room entry and both of the
client's goroutines (`readLoop`, `writePump`) running until something else
tears down the whole game's room. [ADR-0005](../decisions/0005-websocket-protocol.md)
explicitly lists "application-level heartbeat" as out of scope for the
initial protocol design, so this isn't an oversight relative to what was
planned — but it's worth revisiting given how often the described
disconnect actually happens in practice. **Not fixed in this pass.**

### Stage 8 — Bulk Moxfield import and deck resync jobs can get stuck (documented when built, revisited 2026-08-01)

Both `internal/moxfieldimport` and `internal/deckresync` run their import
loop in a bare goroutine with `context.Background()` (deliberate — the
request's context is cancelled the moment the handler returns) and their
own `recover()`. Neither has any mechanism to detect or retry a job left
`in_progress` by a process restart mid-run; the partial unique index that
allows only one active job per user then blocks starting a fresh one,
with no way out short of a manual DB fix. This was an accepted, documented
gap when both features were built ("Accepted, not solved, in this pass" in
the original code comments). It's worth revisiting now that
[ADR-0015](../decisions/0015-infraestructura-de-despliegue.md) puts the
backend on Render's free tier, which sleeps the service between requests —
exactly the kind of interruption this gap doesn't handle.

### Stage 1 — Playgroups: proxy-join and `GET /users/search`

See the 2026-07-28 audit-history entries above for the full narrative —
[ADR-0013](../decisions/0013-proxy-join-y-autorizacion-de-acciones.md)
covers the authorization design in detail. Worth calling out here: the
`GET /users/search?q=` endpoint matches `username` by `ILIKE` (partial,
case-insensitive) but `email` only by **exact** match, deliberately —
partial email matching would let one account enumerate other users'
addresses by prefix/substring. It never returns a third party's email in
the result shape (`UserSearchResult{id, username}`, no `email` field,
unlike the full `UserResponse`), excludes the requester themselves, and
caps results at 10 with a 2-character minimum query.

### Stage 1 — Auth: token strategy and Google Sign-In

Full design rationale in [ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)
(why HS256 over RS256 in a monolith with no external token verifiers) and
[ADR-0002](../decisions/0002-google-sign-in.md) (why `go-oidc` over
`google.golang.org/api/idtoken` — same verification guarantees,
substantially lighter dependency footprint, avoids pulling in all of
`google.golang.org/api` + gRPC + OpenTelemetry to verify one token).
Account linking (`users.FindOrCreateGoogleUser`) looks up by `google_id`
first, then by email with **auto-link only if Google confirms
`email_verified`**, then falls back to creating a new passwordless account
(username derived from the email's local part, retried with a suffix on
collision).

### Stage 4/5 — Known, deliberate Android test gaps

Several `ViewModel`s have no dedicated unit test, all for the same
underlying reason: `SettingsViewModel`, `DashboardViewModel`,
`LoginViewModel`, `RegisterViewModel`, and `JoinGameViewModel` all inject
`SessionManager` directly — a concrete class with a `Context` constructor,
unfakeable in a pure-JVM test without Robolectric (which the project
doesn't use; `testing/Fakes.kt` are hand-written fakes against interfaces
only). Extracting a narrow interface (something like a
`SessionTokenProvider`) would solve it — it was tried and reverted during
the Settings-screen pass, documented there instead of left half-done — but
it's a real architectural change, not part of the domain-layer refactor
that did happen (see the 2026-08-01 domain-layer entry above, which
deliberately left auth out of scope for the same reason).
