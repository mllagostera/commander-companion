# ADR-0001: Authentication strategy — access JWT + rotating opaque refresh token

**Status:** Accepted and implemented (2026-07-26)

## Context

The backend needed real authentication (up to this point, `internal/auth/`
was empty and `internal/users/service.go` used a dummy password hash).
There were decisions to make:

1. How to sign the access tokens: symmetric secret (HS256) vs. an
   asymmetric key pair (RS256/ES256).
2. How to handle session renewal (`POST /auth/refresh`) and real logout
   (`POST /auth/logout` must be able to invalidate a session).
3. Refresh token format: also a JWT, or opaque.

## Decision

- **Access token: JWT signed with HS256**, symmetric secret (`JWT_SECRET`),
  short lifetime (`ACCESS_TOKEN_TTL`, default 15 minutes). Minimal claims:
  `sub` (user ID), `iat`, `exp` (`jwt.RegisteredClaims` from
  `github.com/golang-jwt/jwt/v5`).
- **Refresh token: random opaque string** (32 bytes from `crypto/rand`,
  base64url), **not a JWT**. Only its SHA-256 hash is persisted in the
  `refresh_tokens` table (`token_hash`, `expires_at`, `revoked_at`); the
  plaintext value only exists in the HTTP response, never in the database.
- **Rotation on every use**: `POST /auth/refresh` revokes the refresh token
  used and issues a new one along with the new access token. Reusing a
  refresh token that has already been rotated returns `401`.
- **Real logout**: revokes the specified refresh token (`revoked_at`), does
  not depend on the access token expiring to end the session.
- Refresh token lifetime: `REFRESH_TOKEN_TTL`, default 720h (30 days).

## Alternatives considered

- **RS256/ES256** (key pair): lets external services verify tokens without
  knowing a shared secret. Discarded because the project is a modular
  monolith with no other services that need to verify tokens
  independently — the complexity of managing a key pair (rotation,
  distributing the public key) isn't paid off by any real benefit today.
- **Refresh token also as a JWT**: simpler to generate, but it can't be
  revoked without a revocation list anyway — if state has to be kept in
  the database to allow revocation, it's more direct for the refresh token
  to *be* the pointer to that state directly (opaque + hash), instead of a
  JWT whose contents are never used once you have to go to the database
  anyway.
- **Cookie-based sessions**: discarded because the main client is a native
  Android app (not a browser with trivial same-site cookies), and the
  second client (see [ADR-0004](0004-web-client-nuxt.md)) is a decoupled
  SPA/SSR client that uses Bearer tokens anyway.

## Consequences

- Revoking *all* of a user's sessions (e.g. "log out on all devices")
  requires revoking every `refresh_tokens.user_id` — not implemented yet,
  but the table already supports the query.
- Compromising `JWT_SECRET` invalidates trust in *all* access tokens
  issued until the secret is rotated (and with HS256 there is no way to
  rotate it without invalidating old tokens — no `kid`/multi-secret
  support implemented).
- The default value of `JWT_SECRET` in development
  (`dev-insecure-jwt-secret-change-me`, see `backend/cmd/api/main.go`) is
  intentionally insecure and must be overridden in any real environment.

## References

- Implementation: `backend/internal/auth/token.go`, `service.go`
- Migration: `backend/migrations/00002_auth.sql`
