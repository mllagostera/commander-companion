# ADR-0017: Friend requests (send/accept/reject) and profile QR generation

**Status:** Accepted (2026-08-15) — phases 1-2 of a 3-phase plan implemented (backend + web request lifecycle, web QR generation); phase 3 (Android profile screen, QR generation + camera scanning) is follow-up work, see Consequences.

## Context

The user asked to add a "friends" feature to the app: an authenticated user
should be able to add another either by searching their username, or by
scanning a QR code shown on that user's own profile. This is Stage 9 of the
roadmap ("Social — friends, groups and tournaments"), whose friends item was
still unstarted (`docs/roadmap/TASKS.md`):

> `[ ] Friends system: send/accept/reject requests, friends list — distinct
> from playgroups (game groups, not friendship relations)`

`playgroups` already lets a member add another user directly (by searching
username, `GET /users/search` → `POST /playgroups/{id}/members`), but that
add takes effect immediately with no consent from the person being added.
Friendship needed its own request/accept lifecycle, and the web/Android split
in this repo (web = desktop, decks, statistics; Android = at-the-table,
sub-2-second actions, see README §1) meant QR *scanning* — which only makes
sense with a phone camera — belonged on Android, not web.

Given the size of the full ask (backend, web, and a new Android profile
screen that doesn't exist yet), the work was split into a 3-phase plan,
agreed with the user up front via plan mode: (1) backend + web, request
lifecycle via username search only, no QR; (2) web QR *generation* only
(`settings.vue`, showing your own code); (3) Android — a new profile screen,
QR generation, and QR *scanning*. This ADR covers phases 1 and 2; phase 3 is
still unbuilt.

## Decision

### QR encodes the user's own `id` directly — no new `friend_code`

The key design question was what the QR actually encodes. Two options were
considered: (a) the user's existing `id` (already returned by `GET
/users/me`), or (b) a new rotable `friend_code` column on `users`, analogous
to `tournaments.join_code` (ADR-0016).

(a) was chosen. A `friend_code` would add a column, a regeneration endpoint,
and a "your old QR image no longer works" story, and none of that changes
what an unwanted scan can do: sending a friend request never bypasses
consent, since the addressee still has to accept it. There is no
`join_code`-style "the code alone gets you in" risk to defend against here.
Consequence: **QR generation needs no new backend endpoint at all** — the
client already has its own `id` after login (`AuthUser.id`), and scanning
another user's QR just feeds their `id` into the exact same `POST
/friends/requests` call the username-search flow uses. Both entry points
converge on one endpoint; only the request lifecycle itself is new backend
work. (QR generation/scanning is phases 2-3, not built yet — see Consequences.)

### One `friend_requests` table, no separate `friends` table

`friend_requests` (`requester_id`, `addressee_id`, `status`, migration
`00017_friend_requests.sql`) models the entire lifecycle in one row: an
`accepted` row *is* the friendship, resolved to "the other user" by whichever
side isn't the caller (`ListFriends` query, `CASE WHEN requester_id = ...`).
Same "the row's status models the whole lifecycle" pattern already used by
`tournament_participants`/`moxfield_import_jobs` — a second `friends` table
populated on accept would just be a derived, redundant copy of the same fact.

### Crossed requests auto-accept instead of erroring

If A requests B, and B independently requests A before responding, the
service (`resolveOrCreateRequest`, `internal/friends/service.go`) detects the
existing reverse-direction pending row and accepts it in place, instead of
either erroring or creating a second row that would need its own resolution.
This is checked in the service, not with a DB constraint: the partial unique
index (`friend_requests_pending_direction_idx`) only prevents a *duplicate*
request in the *same* direction, matching the check-then-act pattern already
used by `playgroups.AddMember` (no transaction/lock either) — a genuine
simultaneous race on both directions is an accepted, pre-existing class of
edge case in this codebase, not one this feature needed to solve first.

### Non-addressee/non-requester gets 404, not 403

Accepting or rejecting a request when the caller isn't its addressee (or
cancelling one when the caller isn't its requester) returns
`ErrRequestNotFound` (404), not a 403 — same "don't reveal whether the
resource exists to someone who isn't a party to it" criteria already used by
`playgroups.getMemberPlaygroup` for a non-member's `GetPlaygroup`.

### QR generated as an inline SVG string, not a canvas/PNG

`settings.vue` generates the QR with the `qrcode` npm package's
`QRCode.toString(id, { type: 'svg' })`, rendered via `v-html` into a fixed
white box (regardless of the app's light/dark theme, since a QR needs
reliable black-on-white contrast to scan, not to match the page's palette).
This was chosen over `toDataURL`/canvas-based generation because
`toString('svg')` is pure string generation — no `<canvas>`/`document`
access — so it runs identically during SSR and in the browser, and the QR is
already present in the initial server-rendered HTML instead of popping in
after a client-side effect. No new backend endpoint was needed: the client
already has its own `id` from the session (`useAuth().user.id`), matching
the "no new endpoint for QR generation" consequence already called out in
the Decision section above.

## Alternatives considered

- **`friend_code` rotable column, QR encodes that instead of `id`**: rejected
  per Decision above — no security property it would add, since accept/reject
  already gates every request regardless of how the target was found.
- **Auto-reject the newer request on a same-direction duplicate, silently**:
  rejected in favor of an explicit `409 ErrRequestAlreadyPending` — silently
  dropping a user's action without telling them is worse than a clear error
  they can read a request's existing state and react to.
- **Blocking/muting another user**: out of scope for this phase, not
  designed. A rejected or cancelled request doesn't prevent a future request
  in either direction.
- **Web-side QR scanning (camera)**: out of scope, deferred entirely to
  Android per the phased plan — the web client's whole positioning is
  desktop/decks/statistics, not table-side camera interactions (README §1).

## Consequences

- Phases 1-2 (this ADR) ship a fully usable friends feature through
  `web/app/pages/friends.vue` (search by username,
  send/accept/reject/cancel, friends list, unfriend) plus a QR of your own
  `id` shown in `web/app/pages/settings.vue`. **Nothing can scan it yet** —
  the QR is real and correct (verified end-to-end, see the 2026-08-15
  DECISIONS-LOG entries), but there is no scanner anywhere in the app until
  phase 3 ships. It's only useful today if decoded by some other QR reader
  and the resulting id pasted somewhere — not a real flow for an end user.
- Phase 3 (Android profile screen + QR generation/scanning, new
  `zxing`/CameraX-or-ML-Kit dependencies) is the remaining unbuilt
  follow-up — `docs/roadmap/TASKS.md` tracks it as a separate sub-item under
  Stage 9 rather than marking the whole friends item done.
- Android has no profile screen today at all (confirmed against
  `docs/ux/wireframes.md` during the investigation for this feature) — phase
  3 is what will introduce the first one, driven by this feature's need to
  show a QR and scan one, not as a general profile-screen initiative.
- Blocking/muting is absent: a user who keeps getting requests from the same
  unwanted account has no way to stop them beyond repeatedly rejecting.
  Left as a future addition if it turns out to be needed in practice.

## References

- `backend/migrations/00017_friend_requests.sql`
- `docs/database/schema.dbml` (`friend_requests`)
- `backend/internal/friends/` (`service.go`, `handler.go`, `query.sql`)
- `docs/api/openapi.yaml` (`/friends*` paths and schemas)
- `web/app/pages/friends.vue`, `web/app/composables/useFriends.ts`
- `web/app/pages/settings.vue` (QR generation), `qrcode` npm package
- [ADR-0016](0016-swiss-tournament-format.md) (`join_code`, the precedent
  this ADR's Decision section explains why it wasn't reused as-is)
- [ADR-0013](0013-proxy-join-and-action-authorization.md) (the
  "don't reveal to a non-party" 404 convention reused here)
