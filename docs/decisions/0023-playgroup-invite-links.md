# ADR-0023: Playgroup invite links — one rotatable code per group

**Status:** Accepted (2026-10-04)

## Context

Joining a playgroup required an existing member to find you by username or exact
email (`POST /playgroups/{id}/members`). That works for two or three people, but
the usual way a table forms is someone dropping a link in the group chat. The
request was a public URL, invitation-style, that anyone can open from the web and
use to join.

Constraints that shaped it:

- Every playgroup endpoint is membership-scoped and answers 404 rather than
  revealing a group exists. A link must not open a hole in that.
- Adding a member is already open to **any** member, not only the creator
  (`AddMember`, ADR-0013's proxy-join builds on that).
- The repo already has two token designs: auth tokens (email verification,
  password reset, refresh) stored as a SHA-256 hash, single-use, short TTL; and
  tournament `join_code`s stored in plain text and shown to the organizer.

## Decision

A nullable, unique `playgroups.invite_code` column holding **one code per group**:
128 random bits encoded as 22 base64url characters.

- `POST /playgroups/{id}/invite` creates it or replaces it (the old code stops
  working at once). `DELETE /playgroups/{id}/invite` clears it. **Any member** can
  do either — the same rule as adding a member by hand.
- `GET /playgroup-invites/{code}` previews the group (name, member count, whether
  you're already in) and `POST /playgroup-invites/{code}/accept` joins it. Both
  require a session; an unknown, malformed or revoked code is a plain 404.
- The code is returned as `invite_code` on the `Playgroup` DTO, which only
  members ever receive.
- Web: the group page shows the link with copy / regenerate / disable; the link
  lands on `pages/playgroups/join/[code].vue`, which shows the group and waits for
  a click (no side effect on load, same reasoning as `friends/add/[id].vue`). A
  visitor without a session goes through `/login?redirect=…` and comes back.

Alternatives weighed:

- **A `playgroup_invites` table** (many codes per group, expiry, max uses,
  per-invite revocation, who created it). More flexible, but nothing asked for
  it, and every extra knob is UI the group page has to carry. Moving to it later
  is additive: the column becomes "the default invite".
- **Hash the code, like auth tokens.** Then members can't see the link again,
  only regenerate it, which breaks "copy the link for the new person". Leaking a
  code grants joining one group, not an account, and rotating closes it, so the
  plain-text trade-off is the same one tournaments' `join_code` already makes.
- **A public (sessionless) preview.** It would let the page show the group name
  before login, but it is one more unauthenticated endpoint, and it makes a
  leaked code reveal the group to people who will never sign up. The login detour
  costs one screen.
- **Creator-only management.** Rejected for consistency: a member who can add
  anyone by username gains nothing from being unable to share a link.

## Consequences

- Joining a group is now one click for anyone holding the link, which is the
  point — and also the risk. The only controls are rotate and disable; there is
  no expiry and no use limit. If abuse shows up, a TTL or a member cap is the
  next step, and `playgroup_invites` the shape it would take.
- No rate limiting on the accept/preview endpoints: 128 bits makes guessing
  impractical, and both sit behind auth.
- New users have a gap: registering requires email verification, and the
  verification link does not carry the invite, so a brand-new user has to open
  the invite link again after verifying. Carrying `redirect` through
  registration and verification is a follow-up.
- Android has no invite screen yet: it ignores the new `invite_code` field
  (`ignoreUnknownKeys`), and the URL opens in the browser. An App Link for
  `/playgroups/join/*` is a follow-up, with the same assetlinks caveat as
  ADR-0017's friend QR.
- A member who leaves cannot revoke the link afterwards — but there is no
  "leave group" yet either.
