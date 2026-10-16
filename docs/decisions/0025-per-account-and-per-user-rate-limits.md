# ADR-0025: Rate limit failed logins per account and Moxfield calls per user

**Status:** Accepted (2026-10-16)

## Context

Every rate limit so far is keyed by client IP ([ADR-0024](0024-client-ip-for-rate-limits.md)).
That leaves two gaps:

1. **Distributed password guessing.** `POST /auth/login` allows 20 requests a
   minute per IP. An attacker with many IPs (a botnet, rotating proxies) can try
   passwords against one account without ever hitting that limit.
2. **Moxfield abuse.** Importing or syncing a deck makes outbound calls to
   Moxfield, and the bulk jobs make one per deck. Nothing bounded how often an
   authenticated user could trigger them. Moxfield's limits apply to the API's
   single outbound IP, so one account hammering these endpoints could get that IP
   throttled or blocked, and the feature would stop working for everyone.

## Decision

Two new kinds of limit, kept in memory like the existing ones
(`cmd/api/ratelimit.go`), each keyed by something other than the IP.

**Failed logins per account: 10 per 15 minutes**, keyed by the submitted email
(trimmed, lowercased), on `POST /auth/login`, after the per-IP limit.

- Only failures count (any response ≥ 400). A user who logs in normally never
  spends it.
- The key is the email *as submitted*, whether or not an account exists, and the
  response is the same either way, so the limit reveals nothing about which
  emails are registered.
- A missing or oversized email (over 254 bytes, longer than any valid address)
  falls back to the client IP: such a request fails validation anyway, and an
  arbitrary string must not mint an unbounded number of counters.

**Moxfield calls per user**, keyed by the authenticated user ID:

- **One deck** — `POST /decks/import/moxfield` and `POST /sync/moxfield` share 20
  per 10 minutes. Every request counts: even a `404` already cost a Moxfield call.
- **Bulk jobs** — `POST /moxfield-import` and `POST /decks/resync-all` share 5
  started per hour. Only jobs actually started count: the `400`/`409`
  refusals happen before any Moxfield call. This adds to the existing
  one-job-at-a-time guard, which alone allowed back-to-back jobs indefinitely.

**Counting by outcome needs its own counter** (`outcomeLimiter`). Fiber's
limiter has `SkipSuccessfulRequests`/`SkipFailedRequests`, but they read the
response status right after the handler returns, before `common.ErrorHandler`
turns a returned error into a response. Every handler here returns its errors,
so to Fiber's limiter a failed login is a 200 and would never count — the first
version did exactly that, and its tests caught it. `outcomeLimiter` reads the
status the error will become (`common.MapError`). The budgets that count every
request stay on Fiber's limiter.

The web client and the Android app show a translated "too many attempts" message
for these `429`s instead of a generic error or the backend's English text.

### Alternatives considered

- **Lock the account until its owner acts** (an email link to unlock). Strongest
  against guessing, but turns the throttle into a free tool to lock anyone out
  indefinitely by knowing their email.
- **Keep the counter per account *and* IP.** Doesn't stop the distributed attack
  this exists for.
- **Let a correct password through even while throttled.** Then a throttled
  attacker still learns when a guess is right, which defeats the purpose.
- **CAPTCHA after N failures.** The right next step if the lockout below ever
  bites in practice, but it needs a provider and work on both clients.
- **Persist counters in Postgres.** Survives restarts and replicas, but costs a
  write on every failed login; not justified for a single instance (see
  Consequences).

## Consequences

- **Deliberate trade-off — a targeted lockout.** Anyone who knows a user's email
  can keep that account from logging in with a password by sending 10 wrong
  passwords every 15 minutes. It's temporary (it ends 15 minutes after the
  attacker stops), the per-IP limit still applies to them, and password reset
  isn't affected, but it is real. Accepted because the alternative is unlimited
  distributed guessing. If it happens in practice, add a CAPTCHA rather than
  loosening the limit.
- Counters live in process memory: a deploy or restart resets them, and with
  more than one replica each counts on its own. Same limitation as every other
  limit here ([ADR-0024](0024-client-ip-for-rate-limits.md)).
- A successful login doesn't clear earlier failures; they expire with the
  window. Someone who mistypes 9 times and then gets it right has one failure
  left for the rest of those 15 minutes.
- The Moxfield limits are a guess at legitimate use (a few imports or syncs in a
  sitting, an occasional bulk job). If real users hit them, raise them;
  `Retry-After` tells clients when to retry.

## Addendum (2026-10-16): emails per address

`POST /auth/forgot-password` and `POST /auth/resend-verification` were limited
only per IP (20 a minute), so anyone with a few IPs could make the API mail a
given address over and over: flooding that inbox and spending the Resend quota.
They now share a third per-address budget, **3 per 15 minutes**, keyed by the
submitted email exactly like the failed-login one (same `emailKey`), after the
per-IP limit.

Unlike failed logins, **every request counts**: both endpoints answer `204`
whether or not the account exists (on purpose, so they reveal nothing), which
leaves no failure to tell apart. The same targeted-lockout caveat applies on a
smaller scale: someone who knows an address can use up its 3 emails, delaying a
real reset by up to 15 minutes. Accepted for the same reason as above.

