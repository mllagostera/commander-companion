# ADR-0029: Inbound mail forwarded through Resend

**Status:** Accepted (2026-10-18)

## Context

The web's privacy policy gives `privacidad@tapeandocartones.es` as the contact
for data protection requests (access, erasure...), and GDPR expects that
address to actually reach someone. The domain had no mailbox: Resend only sent
mail (from the `send` subdomain), and DNS lives in Vercel, which hosts no mail.

Resend can also receive mail for a verified domain: an MX record points to it,
it stores every message, and it calls a webhook (`email.received`) with the
metadata. It offers no forwarding rule, though: forwarding is code that fetches
the message and sends a copy.

## Decision

The backend forwards every email Resend receives on the domain to one personal
inbox (`internal/inbound`):

1. `POST /api/v1/webhooks/resend-inbound` verifies the Svix signature Resend
   sends (HMAC-SHA256 with the webhook secret, 5-minute timestamp window),
   implemented with the standard library rather than the Svix SDK.
2. It reads the email from `GET /emails/receiving/{id}`, downloads the original
   from the signed `raw.download_url`, and sends a copy with `POST /emails`:
   the original HTML (or escaped text) under a short header (sender, recipient,
   date, SPF/DKIM/DMARC), `reply_to` set to the original sender, and the
   untouched message attached as `.eml`, which keeps every attachment without
   parsing MIME in Go.
3. The send carries `Idempotency-Key: inbound-forward/<email id>`, so a webhook
   Resend retries after the copy already went out (a slow answer, a manual
   replay) does not deliver it twice. No state is stored on our side.
4. A failure answers 502/504 and Resend retries on its own schedule (immediately,
   5 s, 5 min, 30 min, ...). That schedule also covers the free Render instance
   waking up from sleep on the first delivery.

It lives in the Go backend rather than in the web's Nitro server because the
backend already talks to Resend; the full-access key it needs can still be a
separate variable (`RESEND_INBOUND_API_KEY`), so the sending key keeps its
narrower scope.

Alternatives considered:

- **ImprovMX or a similar forwarding service:** zero code, two DNS records. Lost
  because it adds one more provider with access to the mail, and Resend was
  already in the stack and in the privacy policy.
- **Cloudflare Email Routing:** needs Cloudflare as the domain's nameservers;
  they are Vercel's.
- **A real mailbox (Google Workspace, Zoho...):** monthly cost and another
  account to maintain for a few emails a year.

## Consequences

- Any address on the domain forwards, not just `privacidad@`. The copy's header
  shows which address was used. If spam becomes a problem, filtering by
  recipient is a small change in the handler.
- Replies sent from the personal inbox come from that inbox, not from
  `privacidad@`, unless the inbox is set up to send through Resend's SMTP
  ("Send mail as" in Gmail).
- The domain's apex MX record now belongs to Resend: adding a regular mailbox
  later means moving receiving to a subdomain or replacing this.
- Messages over 25 MB are forwarded without the `.eml` (Resend caps attachments
  at 40 MB after base64); the copy says so, and the original stays in Resend's
  dashboard.
- Startup refuses a half configuration and a forward target on the forwarding
  domain itself, which would loop every copy back through Resend.
