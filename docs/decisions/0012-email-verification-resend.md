# ADR-0012: Email verification at registration with Resend

**Status:** Accepted and implemented (2026-07-28)

## Context

`POST /auth/register` created the account with any email, without checking that
the person registering actually owns that mailbox, and the Nuxt BFF chained an
automatic login that left the session already started. There was no mail
sending library in the backend at all.

What was needed: (1) confirm the email before letting the account operate via
password, just like any standard registration, and (2) a transactional mail
provider whose emails don't end up in spam.

## Decision

### Data model

- `users.email_verified boolean NOT NULL DEFAULT true` (migration
  `00011_email_verification.sql`). Defaults to `true` so as not to have to migrate
  existing users nor touch `CreateUserWithGoogle`: Google already confirms the
  email in its own `id_token` (see [ADR-0002](0002-google-sign-in.md)), so
  only sign-up via email/password (`CreateUser`) forces `false`
  explicitly.
- `email_verification_tokens`, same pattern as `refresh_tokens`
  ([ADR-0001](0001-auth-jwt-refresh-token-strategy.md)): only the SHA-256 hash of
  the token is persisted, never the plaintext value; 24h TTL; single use
  (`used_at`).
- `LinkGoogleID` (linking a Google account to an existing email/password
  account) also marks `email_verified = true`: at that point
  `FindOrCreateGoogleUser` has already confirmed that Google verifies that email, so
  an account not yet confirmed becomes verified through that path too.

### Login policy: blocked until verified

`users.VerifyCredentials` (which `auth.Login` depends on) returns
`ErrEmailNotConfirmed` (`403`) if the password is correct but
`email_verified` is `false`. This is a different code from `ErrInvalidCredentials`
(`401`) because at this point the password has already been proven correct — the client uses
the `403` to offer "resend verification" instead of a generic "check
your credentials."

The alternative of letting the user in with a "verify your
email" banner and restricting some specific action was considered, but there is no
action today that makes sense to restrict halfway, and blocking login is the
simplest pattern to reason about with the current JWT model.

`web/server/api/auth/register.post.ts` no longer chains an automatic login
after registering (the login would fail with `403` anyway): the registration
screen shows a "check your email" message instead of navigating to the dashboard.

### `REQUIRE_EMAIL_VERIFICATION` flag (default `false` in alpha phase)

The project is in alpha phase: it doesn't make sense to spend on sending mail
or to block login for this yet. `config.Config.RequireEmailVerification`
(env var `REQUIRE_EMAIL_VERIFICATION`, default `false`) controls all of the
above behavior from `users.NewService`:

- When `false` (default), `RegisterUser` creates the account with `email_verified =
  true` from the start and **neither generates the token nor calls the `Mailer`** — it does not
  pay the cost of a send that nobody will require. `VerifyCredentials` needs no
  special case: the account is already verified, so
  login works right away.
- When `true`, it's the full flow described above (token, mail, `403` until
  confirmed).

When the project comes out of alpha, it gets turned on with `REQUIRE_EMAIL_VERIFICATION=true`
without touching any code.

### Provider: Resend, with dashboard templates (no HTML in the backend)

**Resend** was chosen for its simple HTTP API and good default deliverability
for a project of this size.

The email content (subject, copy, layout) lives in a **Template** in the
Resend dashboard, not in the backend: `internal/email` just calls
`POST https://api.resend.com/emails` with `template: { id, variables }`
(`USERNAME`, `VERIFY_URL`), without a single line of HTML/text in Go.

**Caveat, verified against an open Resend issue**
([resend/react-email#3247](https://github.com/resend/react-email/issues/3247),
unfixed as of this decision): sending a template via the REST API where a
variable is inside an `href` (button/link) breaks the URL in the actual
send — the "Send test" button in the dashboard doesn't reproduce the bug, so
it's misleading. That's why the template must display `VERIFY_URL` as
**visible plain text**, not as a button: most mail
clients auto-link bare URLs anyway, so the link is still clickable. If Resend
fixes the bug later, it's possible to switch back to a button without touching the
backend — it's a template-only change.

### Console mode without a Resend account

`email.NewResendClient` returns a mailer that just does a `log.Printf` of
the verification link when `RESEND_API_KEY` is empty. This way
`docker-compose up` (and the tests) keep working without anyone needing
a Resend account to develop locally.

## Alternatives considered

- **SendGrid / Amazon SES**: ruled out due to the project owner's preference
  (an existing Resend account) and because SES requires more
  manual domain/reputation configuration to get out of sandbox mode.
- **HTML built in the backend** (`html`/`text` directly in the POST to
  Resend, without `template`): completely avoids the `href` bug
  mentioned above, but means versioning the email copy in the repo instead of the
  Resend dashboard, which is where the project owner prefers
  to keep it. Ruled out in favor of dashboard templates + link as
  plain text.
- **Per-email resend limit** (in addition to the existing per-IP rate limit):
  ruled out as out of scope — the 20 req/min per-IP rate limit that already
  protects all public auth endpoints
  (`cmd/api/main.go: newAuthRateLimiter`) is enough for this change; there is no
  documented abuse case today that would justify it.

## Consequences

- **Pending manual step, outside the code**: for emails not to land
  in spam, a proper domain needs to be verified in the Resend dashboard
  (it generates the SPF/DKIM/DMARC records to add to the domain's DNS), and
  the verification Template needs to be created there (variables `USERNAME` and
  `VERIFY_URL`, published, `VERIFY_URL` as plain text) — see
  `docs/roadmap/TASKS.md`, Auth — email verification section.
- Without `RESEND_API_KEY`/`EMAIL_FROM`/`RESEND_VERIFY_EMAIL_TEMPLATE_ID`
  configured, registration keeps working end-to-end locally (console
  mode), but doesn't send real emails.
- With `REQUIRE_EMAIL_VERIFICATION=false` (default), this entire feature stays
  built but inactive: the column, the tokens table, and the
  `/auth/verify-email`/`/auth/resend-verification` endpoints are still there, but no
  new account needs them. Remember to turn on the flag (and to have
  the Resend domain/template ready) before leaving alpha — noted
  in `docs/roadmap/TASKS.md`.

## References

- Reference source for the Resend Template (pasted as-is into the dashboard,
  Templates → Create template → From code): `0012-verify-email-template.html`
- Implementation: `backend/internal/email/resend.go`,
  `backend/internal/users/service.go` (`RegisterUser`, `VerifyEmail`,
  `ResendVerification`)
- Migration: `backend/migrations/00011_email_verification.sql`
- See also [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (same
  opaque token + SHA-256 hash pattern) and
  [ADR-0002](0002-google-sign-in.md) (why Google accounts already
  end up verified through that path)
