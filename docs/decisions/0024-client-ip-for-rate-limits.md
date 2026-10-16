# ADR-0024: Identify the client IP for rate limits through a BFF secret and an opt-in proxy header

**Status:** Accepted (2026-10-16)

## Context

The API rate limits its public auth endpoints and `GET /users/search` per IP
(Fiber's `limiter`, in-process memory). The key was `fiber.Ctx.IP()`, which with
no `ProxyHeader` configured is the TCP peer address. Two things made that key
wrong in practice:

1. **Every web request comes from the BFF.** The browser never calls the Go API
   (see [ADR-0004](0004-web-client-nuxt.md)): Nitro does, so the API sees Nitro's
   address for every web user. All of them shared one bucket of 20 requests per
   minute for login, register and refresh — and refresh is fired by Nitro
   automatically, so a handful of active users could lock everyone out of
   logging in.
2. **Production sits behind proxies on both sides.** The API runs on Render and
   the web on Vercel ([ADR-0015](0015-deployment-infrastructure.md)), so the TCP
   peer is a platform proxy for direct clients too (the Android app).

A third problem compounded the first: one limiter instance was shared by every
public auth endpoint, so its single per-IP counter was spent by refresh and
login alike.

## Decision

The client IP is resolved once per request by `common.ResolveClientIP`, mounted
before Sentry, and read by every limiter through `common.ClientIP`. Precedence:

1. **The BFF's word, authenticated by a shared secret.** Nitro sends
   `X-BFF-Secret` (`NUXT_BFF_SHARED_SECRET`) and `X-BFF-Client-IP` on every call.
   The API honors the IP only if the secret matches its `BFF_SHARED_SECRET`
   (constant-time compare, at least 32 characters, enforced at startup), and
   strips both headers from every request, matching or not, so Sentry never
   records the secret. Nitro takes the browser's IP from `x-forwarded-for` only
   when running on Vercel, which overwrites that header on every request;
   elsewhere it uses the socket address, because any browser could set the header.
2. **An opt-in edge-proxy header** (`CLIENT_IP_HEADER`) for direct clients. It
   must name a header the proxy overwrites and that holds exactly one IP; a list
   is rejected.
3. **The TCP peer**, as before.

Malformed values fall through to the next step instead of failing the request:
this picks a rate-limit bucket, not an access decision. IPs are canonicalized
(`::ffff:1.2.3.4` → `1.2.3.4`) so one client can't hold two buckets.

The auth limiter is split into two independent budgets: **credentials** (login,
google, register, verify-email, resend-verification, forgot/reset-password,
username-available; 20/min) and **refresh** (60/min). Refresh is looser because
guessing a 256-bit refresh token is not a realistic attack; its limit only
bounds load. `GET /users/search` keeps its own 20/min.

### Alternatives considered

- **Fiber's `TrustedProxies` + `ProxyHeader`**, trusting `X-Forwarded-For` only
  from the BFF's addresses. Lost because Vercel functions have no fixed egress
  IPs, so there is no list to trust.
- **`ProxyHeader: X-Forwarded-For` alone.** Fiber takes the leftmost entry, which
  the client writes; anyone could choose their own bucket and bypass the limit.
- **Signing each request (HMAC of IP + timestamp) instead of sending the
  secret.** Protects against the secret leaking from logs, at the cost of clock
  handling and replay windows. Not worth it once the API strips the header
  before anything records it; revisit if the BFF ever calls the API over a
  channel that logs headers.
- **Requiring `BFF_SHARED_SECRET` when `APP_ENV=production`.** Rejected: without
  it the API still works, only with coarser limits, which is not a reason to
  refuse to start.

## Consequences

- Each web user gets their own budget once both services share the secret. If
  the two values drift apart, nothing breaks loudly: web users silently go back
  to sharing the BFF's bucket.
- Rotating the secret means updating Render and Vercel together; between the
  two deploys web users share a bucket for a few minutes.
- **Render is not resolved by this change.** Whether Render's proxy overwrites a
  single-IP header (`True-Client-IP`, `CF-Connecting-IP`) was not verified: its
  documentation doesn't say, and community reports say it only *appends* to
  `X-Forwarded-For`. Until someone confirms a header (e.g. by checking a request
  that sends a forged value) and sets `CLIENT_IP_HEADER`, direct clients on
  Render — the Android app, and the BFF's own requests when the secret is unset
  — are keyed by Render's proxy address and may share one bucket.
- Counters still live in process memory: with more than one API replica, each
  replica counts separately, and they would need a shared `Storage` (Redis).
