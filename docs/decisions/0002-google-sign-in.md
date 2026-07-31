# ADR-0002: Google Sign-In as an additional authentication provider

**Status:** Accepted and implemented (2026-07-26)

## Context

The original roadmap envisioned email/password as the only login method. It
was decided to add "Sign in with Google" as an additional provider (not as a
replacement) to reduce registration friction, especially on Android where
Credential Manager + Google Identity Services is the flow recommended by
Google.

This forces a decision on: how a Google account relates to an existing
email/password account, and which library is used to verify the `id_token`
in the backend.

## Decision

### Account model

- `users.password_hash` becomes **nullable**; `users.google_id` (varchar,
  unique, nullable) is added. `CHECK (password_hash IS NOT NULL OR
  google_id IS NOT NULL)` at the database level — a user always has *at
  least* one way to authenticate, never none.
- `POST /auth/google` flow (`users.FindOrCreateGoogleUser`):
  1. Looks up by `google_id`. If it exists, direct login.
  2. If not, looks up by `email`. If an account with that email exists **and**
     Google confirms `email_verified: true` in the `id_token`, it is
     **automatically linked** (`LinkGoogleID`) — no additional confirmation
     is requested.
  3. If it doesn't exist either, a new account is created without a
     password, with a username derived from the local part of the email
     (with a suffix if there's a collision).
  4. If Google doesn't confirm `email_verified`, the request is rejected
     (`400`) before looking up/creating anything.

### `id_token` verification library

**`github.com/coreos/go-oidc/v3`** is used instead of the official
`google.golang.org/api/idtoken` library.

## Alternatives considered

- **Requiring manual confirmation when linking by email**: safer against
  the edge case of an email verified by Google that, in practice, isn't
  controlled by the owner of the password account (extremely rare, since
  Google only marks `email_verified: true` after its own verification
  flow). Discarded for now to avoid adding an extra UX step; documented as
  a conscious simplification, not an oversight.
- **`google.golang.org/api/idtoken`** (Google's official library): does
  exactly the same thing (discovery + JWKS + issuer/audience/signature
  validation), but drags along the entire `google.golang.org/api` module
  (`cloud.google.com/go/auth`, gRPC, OpenTelemetry, etc.) — a huge
  dependency tree to verify a token. Confirmed with `go get`: it added
  ~20 new indirect packages. Discarded due to disproportionate weight/attack
  surface relative to the actual problem.
- **Manual JWKS** (`golang-jwt/jwt` + a custom fetch of Google's JWKS): the
  lightest option in theory, but it reimplements key caching/rotation that
  `go-oidc` already handles correctly (it's the standard library in the Go
  community for OIDC verification, with far fewer dependencies than
  Google's official option).

## Consequences

- Google's discovery document (`https://accounts.google.com/.well-known/
  openid-configuration` and its JWKS) is resolved **lazily**, on the first
  Google login, not at server startup — startup doesn't depend on Google
  being reachable.
- Without `GOOGLE_CLIENT_ID` configured, `POST /auth/google` responds `501`
  instead of failing at startup or crashing at runtime.
- Creating the real OAuth credentials (Web Client ID + Android Client ID) in
  Google Cloud Console is a manual, external step that can't be automated
  from the repo — see `docs/roadmap/TASKS.md`, Auth — Google OAuth section.

## References

- Implementation: `backend/internal/auth/google.go`,
  `backend/internal/users/service.go` (`FindOrCreateGoogleUser`)
- Migration: `backend/migrations/00002_auth.sql`
- See also [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (the same
  pair of tokens is issued for login by password or by Google)
