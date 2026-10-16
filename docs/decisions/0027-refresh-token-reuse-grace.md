# ADR-0027: A short grace window for reusing a just-rotated refresh token

**Status:** Accepted (2026-10-16). Amends the reuse-detection rule of
[ADR-0001](0001-auth-jwt-refresh-token-strategy.md).

## Context

Refresh tokens rotate on every use, and since 2026-08-01 presenting an
already-revoked one is treated as theft: `Refresh` revokes every active refresh
token of the account ([ADR-0001](0001-auth-jwt-refresh-token-strategy.md)).

That rule cannot tell a thief from a legitimate client that refreshes twice
with the same token. The second case is routine:

- **Web.** The Nitro BFF de-duplicates concurrent refreshes in memory
  (`inFlightRefresh`, `web/server/utils/backend.ts`), but only within one
  process. On Vercel, two requests carrying the same expired access token can
  land on two instances, and both refresh.
- **Any client.** A refresh whose response is lost (a timeout or a dropped
  connection after the server already rotated) leaves the client holding the
  old token, and its next attempt reuses it. Android's `SingleFlight` only
  collapses concurrent calls within one process; it can't prevent this.

The second refresh arrives a few hundred milliseconds after the first one
rotated the token. Reuse detection then logs the user out of every device over
a timing coincidence. That is bad UX, and it also floods the theft signal with
false positives, so a real theft no longer stands out.

## Decision

A rotated refresh token presented again **within 30 seconds of its rotation,
while the token it was rotated into is still active**, gets a fresh token pair
instead of triggering reuse detection. Everything else keeps the ADR-0001
behaviour.

- **New column `refresh_tokens.replaced_by`** (self-reference). Rotation sets
  it to the successor's ID. Every other revocation leaves it `NULL`: logout,
  reuse detection, a password reset. So only a token that was rotated can ever
  qualify, and only while its successor hasn't been revoked. A replay after the
  legitimate client logged out still revokes the family.
- **The window is checked in SQL** (`HasActiveSuccessorWithinGrace`), against
  the same `now()` that set `revoked_at`, so there is no clock skew between Go
  and Postgres.
- **The grace path issues an independent pair.** The successor already handed
  out isn't recoverable, because only token hashes are stored. Both pairs stay
  valid. The client keeps whichever response it applies last; the other pair
  is never used and expires with its TTL.
- **Rotation is now conditional** (`RotateRefreshToken … WHERE revoked_at IS
  NULL`). When refreshes race on a still-active token, only one records its
  successor; the others still succeed.
- The expiry check and the deactivated-account check (ADR-0018) apply on both
  paths.

### Alternatives considered

- **Return the same successor pair to the second caller.** This needs the
  successor's plaintext, either stored or cached. It is rejected because the
  database must never hold a usable refresh token.
- **A distributed lock in the BFF** (Redis or similar). It fixes only the web
  half, not a retried request from the app, and adds infrastructure for one
  edge case.
- **Drop family revocation.** That reopens the gap ADR-0001 closed: a stolen
  token could be rotated into a fresh session indefinitely.

## Consequences

- **Accepted risk:** a thief who replays a stolen refresh token within 30
  seconds of the legitimate rotation gets a session, without tripping
  detection. This is the trade-off made by Auth0's reuse interval and Okta's
  grace period. A thief who replays later, or after the owner logged out,
  is still caught, and every session of the account is revoked.
- A refresh inside the window leaves one extra active refresh token per
  occurrence. It is harmless (it was only ever sent to the same client) and
  expires on its TTL.
- The tests now age revocations past the window
  (`ageRevocationsPastGrace`) to exercise the theft path. The window itself
  is covered by its own tests in `internal/auth/service_test.go`.
