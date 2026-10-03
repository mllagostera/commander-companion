# ADR-0022: Password reset by email

**Status:** Accepted (2026-10-03)

## Context

An email/password account had no way back in after forgetting its password:
the login page had no "forgot your password?" link, the backend had no reset
endpoint, and `POST /users/:id/password` requires both a session and the
current password. Admins can't reset a password either (ADR-0018). The only
fix was editing the database by hand.

Everything needed was already in place for email verification
([ADR-0012](0012-email-verification-resend.md)): Resend Templates per locale,
the opaque token + SHA-256 hash pattern, the console mailer for local
development, and the per-IP rate limit on public auth endpoints.

## Decision

### Flow

1. `POST /auth/forgot-password` `{email, locale}` — if the email belongs to an
   **active** account, store a new reset token and email the link
   `<WEB_APP_URL>/reset-password?token=…`. It **always answers 204**, the same
   anti-enumeration contract as `/auth/resend-verification`. A failed send is
   only logged, because an error would reveal that the email exists.
2. The web page `/reset-password` reads the token from the query string and
   `POST`s it in the body to `POST /auth/reset-password` `{token, new_password}`
   (POST, not GET, so the token never lands in an access log, same as
   `/auth/verify-email`).
3. In **one transaction** the backend locks the token row (`FOR UPDATE`, so two
   racing requests can't both redeem it), checks that it's unused and
   unexpired, then:
   - sets the new bcrypt hash,
   - marks `email_verified = true` — redeeming a link sent to the address
     proves the user owns it, the same reasoning as `LinkGoogleID`,
   - burns **every** outstanding reset token of the user, so an older email
     left in the inbox stops working too,
   - revokes **all** the user's refresh tokens: whoever had the old password
     may still hold a session.

   An unknown, used or expired token all return the same `400`. A password
   shorter than 8 characters returns `400` *before* touching the token, so the
   user can retry with the same link.

### Data model

`password_reset_tokens` (migration `00024_password_reset_tokens.sql`), the same
shape as `email_verification_tokens`: only the hash is stored, single use
(`used_at`). The TTL is **1 hour** instead of 24h: this link can take over the
account, and the user asked for it moments ago.

### Which accounts get the link

- **Deactivated accounts** (ADR-0018): no email. They couldn't log in after
  resetting anyway.
- **Google-only accounts** (no `password_hash`): they *do* get the link.
  Redeeming it proves they own the mailbox, and it's the only way for those
  accounts to add a password and log in without Google. This doesn't give
  anyone more than they already have: whoever controls the mailbox could
  already reset the password of any email/password account.
- **Unverified accounts**: they get the link, and redeeming it verifies them
  (see above).

### Emails

There's one Resend Template per locale, aliased
`<RESEND_PASSWORD_RESET_TEMPLATE_ID>-<locale>` with variables `USERNAME` and
`RESET_URL`. As with verification, the link is shown as **plain text**, not in
an `href`, because of the Resend bug described in ADR-0012. Unlike
`RESEND_VERIFY_EMAIL_TEMPLATE_ID`, the env var is optional and defaults to
`password-reset`: the reset flow is always on (there's no alpha flag), so it
shouldn't need another env var to work. The reference HTML lives in
`0022-password-reset-template.{es,en,ca}.html`.

### Web

- `login.vue` links to `/forgot-password` and passes the typed email along.
- `/forgot-password` is guest-only. After submitting, it shows "if there's an
  account for X, you'll get a link", never "sent".
- `/reset-password` is public, not guest-only: a link opened in a browser that's
  still signed in has to work too. On success, the Nitro route clears the
  session cookies, because the backend already revoked that session.
- Both pages are `noindex` and disallowed in robots.txt (`shared/seo.ts`).

## Alternatives considered

- **Reset code typed into the app instead of a link**: needs a second
  brute-force guard (a short code has little entropy) and doesn't fit the web
  client, which already handles `/verify-email` links. A link with a 32-byte
  token is the existing pattern.
- **Log the user in automatically after resetting**: saves one step, but it
  would mean issuing tokens from an endpoint that only proves mailbox access,
  bypassing the "deactivated" and future login checks in one place
  (`auth.Login`). Sending the user to `/login` keeps a single entry point.
- **Keep other sessions alive**: rejected. Signing out everywhere is the main
  defense when the reason for the reset is a leaked password.
- **Moving the email send off the request path** (so a known email doesn't
  answer slower than an unknown one): not worth it while
  `POST /auth/register` already reveals whether an email is taken (`409`).

## Consequences

- **The Templates live outside the code**: `password-reset-es`, `-en` and `-ca`
  were created and published in Resend on 2026-10-03. Without them a send fails,
  is only logged, and the user sees the generic "check your email" with nothing
  arriving. Changing their copy means re-uploading from the reference HTML with a
  full-access key, which is kept out of the backend on purpose: its
  `RESEND_API_KEY` stays send-only.
- Without `RESEND_API_KEY` (local, Docker Compose), the reset link is printed
  to the backend log by the console mailer.
- Android only covers the first step: the login screen links to
  `ForgotPasswordScreen`, which calls `/auth/forgot-password`. The new password is
  chosen on the web page the email links to; opening that link in the app instead
  would need Android App Links (a verified `assetlinks.json` on the web domain),
  which isn't worth it for a once-in-a-while flow.
- Reset tokens are never cleaned up, same as verification tokens. At this scale
  the table stays tiny. A periodic purge of expired rows is the follow-up if it
  ever matters.

## References

- Implementation: `backend/internal/users/service.go` (`RequestPasswordReset`,
  `ResetPassword`), `backend/internal/email/resend.go`
  (`SendPasswordResetEmail`), `web/app/pages/forgot-password.vue`,
  `web/app/pages/reset-password.vue`
- Migration: `backend/migrations/00024_password_reset_tokens.sql`
- Email sources: `0022-password-reset-template.es.html`,
  `0022-password-reset-template.en.html`, `0022-password-reset-template.ca.html`
- See also [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (refresh
  tokens being revoked) and [ADR-0012](0012-email-verification-resend.md)
