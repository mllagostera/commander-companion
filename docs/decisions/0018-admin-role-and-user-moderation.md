# ADR-0018: Admin role and user moderation (Phase 1 of the admin dashboard)

**Status:** Accepted (2026-08-17) — Phase 1 of a multi-phase plan implemented
(role foundation, user management, global stats overview). Games/tournaments/
decks moderation are deferred to later phases, see Consequences. Same-day
addendum: online users, active games, and a historical activity chart added
to the overview (see Addendum below).

## Context

The user asked for a full admin "operations panel": users, games,
tournaments, decks, global stats, with moderation actions in each area.
There was no concept of roles anywhere in the app before this — no
`is_admin`/`role` column, no admin-only API, no admin UI. The full ask was
too large for one change, so — agreed with the user up front via plan mode —
the work was split: **Phase 1** (this ADR) builds the role foundation itself
plus the one area needed to prove it end-to-end (user management: list,
search, detail, activate/deactivate) and a small global-stats overview.
Games/tournaments/decks moderation are explicitly out of scope here.

Web only, per the project's own split (README §1): desktop-oriented work
goes to the Nuxt client, table-side/at-a-glance work to Android. An admin
dashboard is squarely the former.

## Decision

### Plain `is_admin` boolean, not a roles table

Considered a `roles`/`user_roles` table for future flexibility (multiple
permission levels), but there is exactly one privilege level needed today —
"can use the admin dashboard or not." A boolean column
(`users.is_admin`, migration `00018_admin_and_account_status.sql`) says
that directly; a roles table would be unused generality. If a second
privilege level is ever needed, migrating a boolean to a roles table is a
contained, well-understood change — not a reason to build it now.

### Account deactivation (`is_active`), not deletion

Moderation in Phase 1 needed *some* action beyond read-only viewing. Hard
deletion was rejected: `decks`, `game_players`, statistics rows, and
tournament participation all reference `users.id`, so deleting a row would
either cascade-destroy other people's game history (anyone who played
against the deleted account) or require nullable-FK workarounds throughout
the schema. A boolean `is_active` (same migration) is reversible, keeps
every foreign key intact, and models "this account can't act right now" —
which is what moderation actually needs — rather than "this account never
existed."

### `is_admin` checked fresh from the DB per request, not embedded in the JWT

The existing JWT (`internal/auth/token.go`) carries only the user ID as
`Subject`, no custom claims — access tokens are short-lived but still valid
for their full TTL regardless of what changes server-side in the meantime
(see `RequireAuth`). Embedding `is_admin` in the token was considered (avoids
a DB round trip on hot paths) but rejected for admin routes specifically:
demoting an admin would leave a stale-privilege window until that token
expires. `auth.RequireAdmin` (chained after `RequireAuth`) instead queries
`users.IsAdmin` on every request. This is an acceptable cost here — admin
traffic is low-volume and not on the sub-two-second gameplay path the rest
of the API is optimized for (README §1) — unlike `RequireAuth` itself, which
stays JWT-only for every other endpoint.

### `is_active` gates login, Google login, and refresh — not already-issued access tokens

The same reasoning almost applied to `is_active`, but full session
revocation (tracking and invalidating every outstanding access token) was
judged too much machinery for what deactivation needs to guarantee. Instead:
`VerifyCredentials` and `FindOrCreateGoogleUser` (`internal/users/service.go`)
reject a deactivated account at login, and `auth.Service.Refresh`
(`internal/auth/service.go`) rejects it too — refresh already re-queries the
refresh-token record, so the check is free there. This bounds a deactivated
account's remaining access to at most one access-token TTL: it can't get a
*new* token, but a token already issued before deactivation keeps working
until it naturally expires. Accepted as a real but small gap for Phase 1,
same "not full session revocation" trade-off already made for refresh-token
theft in ADR-0001, not a new risk class this feature introduces.

### Self-lockout guard on deactivation

`admin.Service.UpdateUserStatus` rejects an admin trying to deactivate their
own account (`ErrCannotDeactivateSelf`, 400). There is no admin-promotion UI
yet (see below), so an admin who locked themselves out would need a manual
DB fix to recover — cheap to prevent up front instead.

### Admin promotion stays a manual DB operation

Phase 1 ships no "make this user an admin" endpoint or UI. The first admin
(and any other, for now) is promoted by hand: `UPDATE users SET is_admin =
true WHERE id = '...'`. Building a self-service promotion flow raises the
question of who's allowed to promote whom, which is exactly the kind of
scope this phase deliberately deferred — see Consequences.

### Global stats: live-computed, one query, no summary table

`GET /admin/stats/overview` (`admin.Service.GetOverviewStats`) runs one SQL
statement with seven correlated subqueries (total/active/verified users,
decks, playgroups, finished games, tournaments) rather than maintaining a
summary table. Same choice already made for `GetPlaygroupStats`
(`internal/statistics`): the query is cheap at this data volume, and a
summary table would need its own invalidation story for a screen that's
viewed rarely.

## Alternatives considered

- **Roles table (`roles`/`user_roles`)**: rejected for now, see Decision —
  no second privilege level exists to justify it yet.
- **Hard-delete a user from the admin panel**: rejected — destroys other
  users' game history via cascading FKs, or needs schema-wide nullable-FK
  changes to avoid it. Deactivation covers the actual moderation need.
- **`is_admin` as a JWT claim**: rejected for admin routes — see Decision,
  stale-privilege window on demotion.
- **Full refresh-token/session revocation on deactivation**: rejected as
  more machinery than Phase 1 needs; the access-token-TTL-bounded exposure
  window was judged an acceptable trade-off, consistent with the existing
  refresh-token-theft handling in ADR-0001.
- **Self-service admin promotion in this phase**: rejected — opens a
  separate authorization question (who can promote whom) out of scope for a
  first pass; manual DB promotion is enough to unblock everything else.

## Addendum (2026-08-17): online users, active games, and a historical activity chart

The overview page gained two more live stats and a chart. Both needed a
definition the app has no infrastructure to answer precisely, so each
required its own trade-off — the same "cheap proxy over exact-but-expensive"
pattern this ADR already uses elsewhere (e.g. `is_admin` checked per request
rather than session-tracked in real time).

**"Online users" = has an unexpired, unrevoked refresh token right now**,
not real-time presence. There's no heartbeat, no websocket-wide connection
registry, and no session table beyond `refresh_tokens` (see ADR-0001) — the
one alternative considered, counting live `internal/websocket` Hub
connections, was rejected because it would only count users currently
inside an active game, undercounting anyone who's logged in but on another
screen (decks, statistics, browsing `/admin` itself). A refresh-token count
(`GetAdminOverviewStats`'s `online_users`) reuses data that already exists,
needs no new tracking, and reads as "has a live session," which is the more
useful operator signal anyway. **"Active games"** is a direct, unambiguous
count of `games.status = 'active'` — no proxy needed.

**The historical chart is derived entirely from `games`/`game_players`, not
a new daily-snapshot table.** A snapshot table (write one row per day with
that day's online-user/active-game counts) was rejected because this
backend has no scheduler (`cmd/api/main.go` only runs one-off startup
tasks, see the `reapStaleBackgroundJobs` call) — a "run once a day" job
would be new infrastructure just for this chart, and the chart would start
empty with no history until it had run for a while. Instead,
`admin.Service.GetDailyActivity` groups `games`/`game_players` by
`date_trunc('day', started_at)`: per day, count of games started and count
of distinct users who played one. This means the chart's "active users"
series is retroactive gameplay activity ("played a game that day"), a
different (and narrower) definition than the overview card's "online users"
(has a session right now) — they intentionally don't share a definition,
since one is a live snapshot and the other is history the app can actually
reconstruct. Days with no games don't produce a row from the query; the
service fills every day in the requested range to zero so the chart always
gets a continuous series. `days` (default 30) is clamped to `[1, 90]`
server-side so an arbitrary query param can't force an unbounded scan.

## Consequences

- Phase 1 ships `backend/internal/admin` (`GET /admin/users`,
  `GET /admin/users/{id}`, `PATCH /admin/users/{id}/status`,
  `GET /admin/stats/overview`, `GET /admin/stats/activity`, all behind
  `auth.RequireAdmin`), the `is_admin`/`is_active` columns, and
  `web/app/pages/admin/` (overview with online users/active games/historical
  activity chart, user list, user detail with activate/deactivate).
- **Deferred to later phases** (per the roadmap's phased plan agreed with the
  user): moderation of games, tournaments, and decks; an admin-promotion
  UI/flow; an audit log of admin actions; full session/token revocation on
  deactivation (bounded instead to one access-token TTL, see Decision);
  Android admin screens (this feature is web-only by design, see Context).
- A deactivated account with an already-issued access token keeps working
  until that token's natural expiry — a known, accepted gap (see Decision),
  not a bug.
- `docs/roadmap/TASKS.md` tracks the admin dashboard as a new item under a
  dedicated section, with Phase 1 marked done and the deferred phases listed
  as open sub-items.

## References

- `backend/migrations/00018_admin_and_account_status.sql`
- `docs/database/schema.dbml` (`users.is_admin`, `users.is_active`)
- `backend/internal/admin/` (`service.go`, `handler.go`, `query.sql`)
- `backend/internal/auth/middleware.go` (`RequireAdmin`)
- `backend/internal/users/service.go` (`ErrAccountDeactivated`, the
  login/Google-login is_active checks)
- `backend/internal/auth/service.go` (`Refresh`'s is_active check)
- `docs/api/openapi.yaml` (`/admin/*` paths and schemas)
- `web/app/pages/admin/`, `web/app/middleware/admin.ts`,
  `web/app/composables/useAdmin.ts`
- [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (the JWT/refresh-token
  design this ADR builds on, including the precedent for bounding an
  exposure window instead of building full revocation)
