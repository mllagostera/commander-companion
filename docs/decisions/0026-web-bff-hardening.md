# ADR-0026: Web BFF hardening — CSP scope, cross-origin check, Nitro-side limits

**Status:** Accepted (2026-10-16)

## Context

A security review of the web client (2026-10-16) found four weaknesses in the
Nitro BFF. None was exploitable on its own today, but each one weakened a
guarantee the code claimed to give:

1. **The CSP allowed injected inline scripts.** `script-src` hashed *every*
   inline `<script>` in the final HTML instead of using `'unsafe-inline'`. A
   script an attacker managed to inject into the server-rendered page (a future
   `v-html`, unescaped user content) was hashed from the same response and
   therefore allowed. The comment said the opposite.
2. **No cross-origin check on state-changing calls.** The session cookies are
   `SameSite=Lax`, which keeps them off cross-*site* POSTs. But "site" is the
   registrable domain: a page on a sibling subdomain is same-site, and its
   POSTs to `/api/**` would carry the cookies.
3. **`/api/scryfall/commanders` had no limits.** Anonymous, no rate limit, no
   query cap. It is the one BFF endpoint that reaches a third party without
   going through the Go API's rate limits, and Scryfall throttles (then bans)
   by IP — ours.
4. **The `/api/backend/**` proxy filtered encoded paths.** h3 passes the
   catch-all param still percent-encoded, and the WHATWG URL parser behind
   `$fetch` treats `%2e%2e` as `..`. So `/api/backend/%2e%2e/v1/auth/me`
   got past the `auth/` block and could reach anything outside `/api/v1`.

## Decision

**1. Hash only the scripts outside the app-rendered region.** A Nitro
`render:html` hook records the inline scripts found in the chunks Vue rendered
(`bodyPrepend` + `body`: the `#__nuxt` root and teleports). Then
`beforeResponse` hashes the executable inline scripts in the final HTML,
leaving those out. Nuxt never puts its own inline scripts in that region (the
runtime-config script is at `bodyClose`, the importmap is in `<head>`). So
anything found there can only come from rendered content, and it stays
blocked. Data blocks (`application/json`, `application/ld+json`) are no longer
hashed at all: the browser never executes them.

**2. Reject cross-origin unsafe requests to `/api/**`** (`server/middleware/csrf.ts`).
`Sec-Fetch-Site` is the signal, and anything but `same-origin` is rejected with
a 403. When that header is absent, `Origin` must equal the request's own origin.
When both are absent, the request isn't coming from a browser page, so it can't
be using a victim's cookies, and it is let through. Safe methods are exempt.

**3. Limit the Scryfall typeahead in Nitro.** It requires session cookies to be
present (a presence check, not verification). It caps `q` at 100 characters.
It allows 60 requests per minute per browser IP, and 8 upstream calls per second
per instance (cache misses only). The limiter (`server/utils/rate-limit.ts`)
is an in-memory fixed window.

**4. Decode, validate, re-encode proxy path segments** before the `auth/`
check. Dot segments are dropped. Segments that are undecodable or that hide a
`/`, `\` or `%` are rejected with a 404. Each surviving segment is
re-encoded, so the path the filter checked is the path the API receives.

### Alternatives considered

- **Per-request CSP nonces** (e.g. `nuxt-security`). This is the textbook fix,
  but a new dependency that takes over every security header. Nuxt 4.5 only
  propagates nonces in its streaming renderer, which this app doesn't use. And a
  nonce added after rendering, by rewriting `<script` tags, has exactly the flaw
  being fixed here. Worth revisiting if Nuxt gains first-class nonce support for
  the non-streaming renderer.
- **Synchronizer CSRF tokens.** They need state or a signed cookie, plus a
  header on every client call. Fetch metadata gives the same protection
  against the threat here (same-site pages) without changing the client.
- **Verifying the session for the Scryfall endpoint** (`/auth/me` per call).
  That costs a Go API round trip per keystroke for a typeahead. The presence
  check stops crawlers and drive-by abuse. The rate limits are what bound a
  determined caller.

## Consequences

- A `useHead()` inline script with `tagPosition: 'bodyOpen'` would land in the
  app-rendered region and be blocked by the CSP. Keep inline scripts in the
  head or at bodyClose.
- Rate limits in Nitro are per instance. On Vercel, several warm instances each
  keep their own counters, so the effective limits are a multiple of the nominal
  ones. That is enough to make abuse expensive and to keep any one instance
  under Scryfall's rate. It is not an exact quota.
- A non-browser client that sends a mismatching `Origin` on purpose gets a
  403. No first-party client does: Android talks to the Go API directly.
- A literal `%` can no longer appear in a proxied path segment. No route in
  `openapi.yaml` takes one.
