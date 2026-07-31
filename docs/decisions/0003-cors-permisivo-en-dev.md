# ADR-0003: CORS open by default, restrictable via environment variable

**Status:** Accepted and implemented (2026-07-26)

## Context

The backend had no CORS middleware. While the only client was Android
(which is not subject to same-origin policy), this wasn't a problem, but it
stopped being one as soon as the first browser-based client appeared:
`tools/auth-test/` (a manual test tool for the auth flow) and, in the
future, the Nuxt client (see [ADR-0004](0004-web-client-nuxt.md)).

## Decision

`github.com/gofiber/fiber/v2/middleware/cors` is added, with allowed
origins read from `CORS_ALLOWED_ORIGINS` (a comma-separated list). If the
variable is empty, **any origin is allowed by default (`*`)**.

This is safe because the API **never uses cookies** for authentication —
everything is a Bearer token in the `Authorization` header, which a
malicious origin can't read or automatically attach to cross-site requests
the way it could with cookies. An `Access-Control-Allow-Origin: *` doesn't
expose anything an attacker couldn't obtain the same way by making a
server-to-server request (the API is already designed to be public/consumed
by multiple clients).

## Alternatives considered

- **Mandatory whitelist from day one**: apparently "safer", but in practice
  only adds configuration friction in dev/testing (every new local static
  server port would require touching the env var) without mitigating a
  real risk, given that no cookies are involved.
- **Dynamically reflecting the received `Origin` without a list**:
  equivalent in practice to `*` for this case (without credentials), but
  more code for the same result.

## Consequences

- In any environment other than local development, `CORS_ALLOWED_ORIGINS`
  must be set explicitly to the real origins (the Nuxt client's domain in
  production, etc.) — the open default is intentionally only for dev.
- If cookie-based authentication is added in the future (for example, a
  refresh token in an `HttpOnly` cookie for the web client), this decision
  will need to be revisited: `AllowOrigins: "*"` is incompatible with
  `AllowCredentials: true` per the CORS specification, and in that scenario
  a real whitelist would indeed be needed.

## References

- Implementation: `backend/cmd/api/main.go` (`corsAllowedOrigins`)
- `backend/.env.example`
