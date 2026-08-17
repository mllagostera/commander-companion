# Commander Companion — Decisions & History Log

Companion to [TASKS.md](TASKS.md). TASKS.md is a compact status checklist —
what's done, what's not, one line each. This file is the narrative behind
it: what changed, why, gotchas hit along the way, how it was verified,
dates, PRs, and the exact user requests that triggered each pass. It exists
because that narrative is genuinely useful project history (TASKS.md's own
rules say not to delete it) but loading all of it on every new session was
the single biggest avoidable token cost in this repo — as of 2026-08-01,
TASKS.md's "Last reviewed" field alone was a single ~30,000-character line.

This file is split in two:
- **Audit / session history** — the chronological log of review passes, in
  the reverse-chronological "newest first" convention the project already
  used, condensed from what used to be TASKS.md's "Last reviewed" field.
- **Per-stage narrative** — organized under the same Stage headers as
  TASKS.md, one entry per non-trivial item, for the detail that didn't fit
  in a single status line.

Not every TASKS.md item has an entry here — short, self-explanatory items
don't need one. When TASKS.md links here for an item, the corresponding
Stage section below has the detail.

---

## Audit / session history (newest first)

**2026-08-17 — Friends phase 3 on Android, and what investigating it changed
about the plan.** The user asked for a plan for the Android phase, chose
Google's code scanner over CameraX, and picked a deep link over the bare UUID.
Reading the Android client first moved two of the plan's load-bearing pieces.

- **The "new profile screen" that ADR-0017 sized phase 3 around was not
  needed.** The web client doesn't put the QR on a profile either — it's a card
  in `settings.vue` — and Android's `SettingsViewModel` already loads the full
  `UserDto` (id included) through `AuthApi.me()`. The card dropped into the
  settings screen that already existed. The single biggest estimated chunk of
  the phase didn't exist.
- **What the plan was missing instead: a friends screen.** Scanning creates a
  *pending* request the other person has to accept, so with only a scanner the
  person being added would have had to open the web client to accept it —
  phone in hand, at the table. `GET /users/search` was also absent from
  Android's API surface, and the username path needs it.
- **The deep link pulled the work outside Android.** The QR is a contract
  between two clients, so the web had to emit the new format before Android
  could consume it. That meant changing `settings.vue`'s payload, adding
  `web/app/pages/friends/add/[id].vue` as the landing page, and two things that
  only showed up while building it: `pages/friends.vue` had to become
  `pages/friends/index.vue` (leaving it beside a `pages/friends/` directory
  would have made it the parent route and the child would have rendered
  nothing), and the auth middleware was **discarding the destination** when it
  bounced you to login, so a deep link opened without a session was consumed
  and lost. It now travels as `?redirect=`, restricted to same-site absolute
  paths — a query parameter that accepts `//evil.com` turns the login page into
  an open redirect.
- **The landing page deliberately does not send on load.** A URL can be
  prefetched, opened by accident, or come from someone else's screenshot.

**The verification story is the unusual part of this pass.** This environment's
network policy blocks `dl.google.com`, and every route to the Android Gradle
Plugin redirects there (`maven.google.com` 301s to it), so the Android module
cannot even be *configured* locally — no compile, no unit tests. Maven Central
is reachable; Google's Maven is not. `android-ci.yml` only runs on `push` to
`main` and on `pull_request`, so a branch push doesn't build either: PR #91 was
opened primarily to obtain a compiler, and each phase was pushed and verified
there before the next was written, rather than accumulating unverified Kotlin.
Checking the job logs rather than the green tick mattered — the point was
whether `compileDebugUnitTestKotlin` and `testDebugUnitTest` actually
*executed* (they did, and the uploaded test-results artifact grew by one file
per new test class).

Two things were designed specifically to be testable without a device:

- **`QrEncoder` holds every part with logic and no Android type at all.**
  ZXing's `core` is pure Java, so building the link, encoding the matrix and
  parsing a scanned code all run on the JVM. The only untestable step — matrix
  to `Bitmap` — is four lines in the UI layer. That split allows the strongest
  check available without a camera: the test encodes the link, **decodes the
  matrix back with ZXing's own reader**, and asserts the text survived. A QR
  that renders but decodes wrong would otherwise only surface on a phone. The
  same was done on the web side by regenerating the SVG from the expected URL
  and comparing path data.
- **`onScanned()` lives in the ViewModel, not the screen**, so the rejection
  path is covered: a Wi-Fi QR, a product barcode or an unrelated URL set
  `INVALID_CODE` without spending a request, and cancelling the scanner (which
  arrives as null) takes the same path.

Smaller decisions worth keeping: the ViewModel returns a `FriendsError` enum
and the screen resolves it to a string resource, because the app ships three
locales and a literal in the ViewModel would be untranslatable
(`ApiError.toUserMessage()` does hardcode Spanish; it predates the other two
locales and was deliberately not followed). 409 is mapped by context — "already
friends / already pending" when sending versus "no longer pending" when
responding are the same status with opposite meanings. And `FriendsViewModel`
depends only on `FriendsRepository`, never on `SessionManager`, which is
precisely why five other ViewModels have no tests (see "Known, deliberate
Android test gaps" below) — avoiding that inheritance cost nothing and bought
18 tests.

Two test details came from `StandardTestDispatcher` rather than from the code:
the busy-row test was asserting state from before its coroutine had started
(now gated on a `CompletableDeferred`), and `advanceTimeBy` needs a `Long`.

**Left open, and only the repo owner can close it:** App Links *verification*
needs `/.well-known/assetlinks.json` carrying the release keystore's SHA-256
fingerprint. Until it exists the deep link still works — it opens the web page,
which does the same thing — but not *directly in the app*. A placeholder was
deliberately not committed, since it would serve an invalid file in production.
Android release builds also need `-PWEB_APP_URL=https://<domain>`; the default
is the emulator alias, and a release built without it would generate QRs
pointing at localhost.


**2026-08-16 — Creating decks by hand, and where the Scryfall lookup had to
live.** The user asked for a way to create a deck from the decks page with a
name and a commander, the commander picked from a Scryfall-backed dropdown of
legendary creatures. Investigating first changed the shape of the work: the Go
API's `POST /decks` has existed since Stage 1 with exactly the right fields
(`name`, `commander`, `image_url`) and **no client had ever called it** — the
web's "Add deck" button only opened the Moxfield import, so a deck that isn't
on Moxfield could not be added at all. So the backend work was near zero and
the real questions were about the lookup.

- **The lookup runs in Nitro, not the browser, and three independent things
  forced that.** The app's own CSP sets `connect-src 'self'`
  (server/utils/security-headers.ts), so a `fetch` to api.scryfall.com from
  the page is simply blocked — and widening connect-src to a third party to
  power a form field is a bad trade. Scryfall's guidelines also ask clients to
  identify themselves with a `User-Agent`, which a browser cannot set and Node
  can. And `/cards/search` answers with *full* card objects — every printing,
  legality and price — so a browser-side typeahead would download roughly two
  orders of magnitude more bytes per keystroke than the two fields a dropdown
  needs. Nitro solves all three at once and keeps `connect-src` untouched;
  only `img-src` had to gain `https://cards.scryfall.io`, for the art.
- **`is:commander` rather than "legendary creature".** The request said
  legendary creatures, but that's both too wide (not every legendary creature
  can lead a deck) and too narrow (planeswalkers and Backgrounds that say they
  can). Scryfall has a predicate for exactly this concept, so the query is
  `is:commander name:"…"`, ordered by `order=edhrec` — popularity ranking is
  what makes a 10-item dropdown useful instead of alphabetical noise — and
  `unique=cards` so reprints don't fill it with duplicates.
- **Two Scryfall behaviours the code has to know about**: a search that matches
  nothing answers **404**, not an empty 200 — for a typeahead that's the normal
  state of a half-typed name, so it maps to `[]` rather than an error — and
  double-faced cards carry no top-level `image_uris`, their art hanging off
  `card_faces[0]`. Both are covered by the endpoint and were exercised in
  verification.
- **The art is attached by derivation, not by a watcher.** Picking a suggestion
  emits `update:modelValue` and `select` in the same tick, so a default
  (`flush: 'pre'`) watcher clearing the art on edit would run *after* the pick
  set it and wipe it every time. It's a computed keyed on whether the field
  still holds the picked commander's name, which has no ordering hazard and
  says the rule out loud: edit the name by hand and the art goes away.
- **Verification had to work around the sandbox.** This environment's network
  policy blocks `api.scryfall.com` (confirmed via the proxy's own status
  endpoint), so a local fake mimicking Scryfall's response shape — including a
  double-faced card, an art-less card, and the 404-on-no-matches — was stood up
  and the endpoint temporarily pointed at it. That verified the real parsing,
  the query actually sent (`is:commander name:"atra"`, `order=edhrec`,
  `unique=cards`, `User-Agent` present), the art fallback and the 404 mapping,
  plus 24 browser checks over the full flow. **The one thing it cannot prove is
  the live contract with Scryfall itself**, which needs a manual check from a
  machine with network access.
- **Two pieces of copy were left saying the wrong thing** and only the render
  showed it: the page subtitle read "Imported from Moxfield." and the empty
  state said "Import a deck from Moxfield to start tracking…" while its CTA now
  opened the manual form. Both rewritten to cover either route.
- **Blank name/commander had never been validated.** `CreateDeck` took whatever
  it was given; the Moxfield import always fills both, so it was unreachable
  until this form existed. Now trimmed and required like playgroups' and
  tournaments' names, with a table test and the 400 documented in
  `openapi.yaml`. The modal's two close buttons also measured 12×20 — under SC
  2.5.8's floor, missed by the earlier target-size sweep because that audit
  never opened a modal.

**2026-08-16 — Closing the web pass: the playgroup ranking's empty state, and
the radius scale everywhere else.** The two items the design review had left
open, finished in one pass.

- **The ranking empty state was solved by moving it, not restyling it.** The
  reason it had been held back is that it was being rendered *as a row inside*
  the bordered ranking table, where a dashed card reads as a broken row. But
  with no ranked members the table has nothing in it but its header — so the
  empty state belongs *instead of* the table, not inside it. It's now an
  `v-if`/`v-else` against the whole container, which is what made it look
  right without any new styling.
- **Then screenshotting it surfaced a second problem the code review hadn't.**
  On an empty group the page showed two identical "New game" CTAs one scroll
  apart — because the ranking is derived from games, so it is empty in exactly
  the cases where the history below it is, and that section already owned the
  action. The ranking's empty state is now deliberately CTA-less (`EmptyState`
  already treats the CTA as optional). Worth recording as the reason: the
  duplication is invisible in the diff and obvious in the render, especially at
  390px where both cards are in the same viewport.
- **The radius scale was applied by role, not by find-and-replace.** The four
  tokens (`--radius-xl/lg/md/sm`, 28/22/16/10px) were introduced with the
  dashboard rebuild but only used there. Rolling them out to the remaining
  files meant deciding what each surface *is* — xl for page-level and auth
  cards and modals, lg for section cards and table containers, md for rows,
  popups and inner blocks, sm for dropdown options, thumbnails and chips —
  rather than mapping old pixel values one-to-one, since the old values were
  themselves inconsistent. Zero hardcoded card radii remain. What deliberately
  stayed literal: the percentage-based organic shapes (the background blobs'
  `63%_37%_54%_46%/…`, the logo glyph), `play.vue`'s `rounded-sm` bars, which
  are a pause *icon* and not a card, and its asymmetric winner badge.

Verified with `npm run lint`, `nuxt typecheck` and `npm run build` all clean,
the scripted audit re-run at 0 issues across nine screens × two viewports, i18n
parity held at 443 keys in all three locales (`ranking.empty` replaced by
`emptyTitle`/`emptyBody`, old key referenced nowhere), and screenshots of a
purpose-made empty group at 1440px and 390px.

**2026-08-16 — Design review across every screen, and the ten fixes it
produced.** The user asked for a sweep of the whole web client. Rather than
eyeball it, the pass combined a scripted audit (horizontal overflow, clipped
text, WCAG 2.2 target sizes, missing accessible names, run over all nine
screens at 1440px and 390px) with a visual read of each screen against
**seeded** data — tournaments and friends had never been looked at with
content in them, so a tournament was created through the real API (8 guests,
started, one table result recorded) and friend rows were inserted directly.

The structural audit came back healthier than expected: no horizontal
overflow, no truncation, no unnamed controls. Its findings were mostly noise
(the visually-hidden skip link, and nav links that WCAG 2.2 SC 2.5.8 exempts
as inline text), which is why the filtered result — 14 real issues, then 0
after the fixes — is the number worth keeping. What the visual read caught
was more interesting than what the script did:

- **The tournament join code was hidden exactly when it becomes useful.** It
  rendered only while `status === 'registration'`, but
  `GET /tournaments/lookup?code=` is what a participant uses to find their
  table *each round*. Now shown until the tournament is finished, with a hint
  that changes by phase.
- **Recorded tables read as "#1 #2 #4 #3"** — seats were rendered in the
  API's seat order. They're now sorted by finishing position once a result is
  in, and left in seat order while it isn't (that's the physical seating,
  which is what helps the organizer fill the form).
- **A player who left a playgroup rendered as a raw UUID** in its ranking and
  history (`usernameFor`'s `?? userId` fallback). Now a "former member" label.
- **`EmptyState` had only ever been used by the dashboard**, so eight other
  empty states were still bare grey sentences — the exact thing that
  component was introduced to fix. Migrated, with the component gaining a
  button mode (`@cta`) for the ones that resolve in place rather than by
  navigating, e.g. opening the deck-import modal. The playgroup ranking's was
  the awkward one — a dashed card nested inside the bordered table read as a
  broken row — and it was held back a pass before being solved properly (see
  the follow-up below).
- **The position pickers were native `<select>`s** — the only browser-painted
  control left in the app, which `SortSelect` exists precisely to avoid (see
  its own doc comment). Swapped.
- **Amber had no token** and `#fbbf24` measures 1.48:1 on a light card, so
  the "in progress" status was another light-theme casualty: added
  `--warn`/`--warn-bg` (light `#7c4a06`, 5.66:1) and made that status a pill
  like every other status in the app.
- **The friends page repeated the old dashboard's mistake** — full-width rows
  carrying a name and two buttons, ~800px of dead space between them, and
  avatars on the friends list but not on the requests. Requests now sit in a
  two-column row with avatars, via a new `UserAvatar` component that derives
  its colour from the whole username (the old inline version keyed off
  `username.length`, handing every five-letter name the same colour).
- **Unfriending was a bare text link that took effect instantly.** Now a real
  button behind a confirmation dialog, verified end to end: Escape cancels
  without removing anyone, confirming actually removes.
- **Five 20px-tall targets** (section "view all" links, "back to
  tournaments", play's "cancel") were under SC 2.5.8's 24px floor — given
  padded hit areas with negative margins so nothing moves visually.

One finding was investigated and dismissed rather than "fixed": the deck
cards looked like white text straight on card art, but they already carry a
left-to-right scrim (0.94 → 0) under exactly the region the text occupies.
Worth recording because the screenshot was misleading and the placeholder
art made it look worse than it is.

**2026-08-15 — Dashboard rebuilt (structure, density and empty states).**
The user asked whether the main dashboard could be improved visually. Judging
it fairly needed data: the gallery's own `dashboard-with-activity.png` was
captured on an account with a single empty playgroup, so it showed the same
"nothing yet" text as the empty-state shot. A throwaway account was seeded
directly in the local Postgres (14 finished games across 2 playgroups, 5
decks with stats summaries) to see the screen as a real user would. Four
concrete problems came out of that, and all four drove the redesign:

- **Radii had drifted with no scale** — 18/20/22/24/26/28px, including three
  cards sitting side by side in the same grid at 24, 20 and 26. Fixed by
  adding a four-step `--radius-{sm,md,lg,xl}` scale to `main.css` alongside
  the existing colour tokens (deliberately outside the `[data-theme]` blocks:
  radii don't vary by theme). The dashboard uses it throughout; other pages
  can adopt it opportunistically, which is why the tokens were introduced as
  a scale rather than as a repo-wide rename in this pass.
- **The second row restated the first** — a KPI strip said win rate / wins /
  games, and the card immediately below repeated all three around the ring.
  Resolved by dropping the KPI strip and letting a single "performance" card
  own win rate (ring), wins-losses, streak, and a totals footer.
- **`Recent games` wasted its width** — each row was `date · group` and a
  result badge with ~1000px of nothing between them, despite the payload
  already carrying each game's players and `deck_id`. Rows now show the deck
  played (with its art) and the opponents' usernames; no new endpoint was
  needed, since the usernames come from the playgroup members already
  fetched for the groups section (`Game.players` only carries `user_id`).
- **Empty states were five grey sentences with nothing to click** — the
  literal first impression of the app. Replaced with a shared `EmptyState`
  component (dashed border, title, body, action) so each section points at
  what to do next: import a deck, start a game, create a group.

The best deck became the page's visual anchor: a spotlight card rendering its
own art full-bleed behind the text. That needed a `fill` mode on `DeckArt`,
and produced the one real bug of the session — passing `absolute` from the
parent did nothing, because `DeckArt`'s root already carried `relative` and
Tailwind emits `.relative` *after* `.absolute`, so on equal specificity the
component's own class won regardless of the order they appear in the class
attribute. Caught by inspecting the live DOM (the art was rendering as a
91px-tall strip) rather than by eyeballing the screenshot; `fill` now sets
the positioning itself instead of relying on the caller.

Verified against the running stack (local Postgres + `go run ./cmd/api` +
`nuxt dev`) with real browser captures at 1440px and at a 390px phone
viewport, in both themes, and in both the populated and brand-new-account
states — the phone pass caught deck names truncating to "Atraxa Su…" in
`Recent games`, fixed by dropping the group name from that column below
`sm`. `eslint` / `nuxt typecheck` / a real `nuxt build` all clean, i18n keys
re-checked for parity across the three locales (427 each) with no orphaned
or missing dashboard keys. `docs/ux/screenshots.md`'s two Dashboard captures
were regenerated (in `es`, 1440×900, matching the rest of the gallery) since
the README asks for that after a significant UI change.

**Light-theme contrast pass (same session, after the user reported the light
theme was hard to read).** Measured rather than eyeballed: a script walked
every text node on the dashboard, composited each one's real painted
backdrop from a screenshot pixel, and computed WCAG ratios in both themes.
The dark theme passed AA everywhere (`--text-dim` at 5.37:1); the light
theme's equivalents came in at **4.29:1** — under AA's 4.5 for body text —
so every secondary line (opponents, dates, commanders, counts, totals) was
genuinely worse in light than in dark. `--win` on `--win-bg` was 4.15. The
fix is at the token level, so it lands across the whole app rather than just
this screen: `--text-dim` → `#625f76`, `--text-muted` → `#524f64`, `--win` →
`#0c625b`, `--lose` → `#ac1a1a`, each solved for ≥5.4 against the measured
light card surface (`rgb(242,241,245)`) so light now matches dark's headroom
instead of merely scraping the minimum.

Three separate bugs of the same family came out of that pass:

- **Surfaces hardcoded as translucent white**, invisible on a near-white
  page: the win-rate ring's track (`rgba(255,255,255,0.08)`, so the ring
  rendered as a floating arc with nothing behind it — now the `--ring-track`
  token), and the ranking-table headers/rows in `playgroups/[id].vue` and
  `tournaments/[id].vue`, moved to `--dim-bg`/`--card-border`/`--card-bg`.
- **`statistics.vue`'s win-rate figure** was a hardcoded `#e9b8fb`, which is
  **1.47:1** on a light card — effectively unreadable. Now `--accent-link`.
- **The inverse mistake, in the new dashboard's own spotlight card**: that
  card is dark in *both* themes (its overlay sits on the deck art), but it
  was pulling `--accent-link`/`--win`, which are *dark* colours in the light
  theme — 3.38:1 and 2.67:1 against their own card. Fixed with deliberate
  light-on-dark literals there, commented so the next person doesn't
  "helpfully" tokenise them back.

That last one is the trap in both directions, and it nearly bit twice: an
initial sweep also replaced the hardcoded greys in `play.vue` with tokens,
which would have broken them — the life tracker renders inside a
`fixed inset-0` overlay with a hardcoded `#0a0714` background, so it is
always dark and its literals are correct. That change was reverted after
checking where the overlay actually starts (line 176; the four token uses
above it are the theme-aware setup screen and are fine). Rule of thumb for
this codebase: theme tokens belong on theme-aware surfaces, literals belong
on surfaces that are pinned to one theme — and the only way to tell them
apart is to find which ancestor paints the background.

Final state: of 48 text nodes sampled per theme, the only remaining
sub-AA readings are measurement artefacts — the wordmark uses
`background-clip: text` (so its computed `color` isn't what's painted), and
the "+ New game" pill and avatar sample their backdrop just outside the
pill; checked against their actual purple gradient they measure 4.69 and
4.83. `eslint`/`typecheck`/`nuxt build` clean, and the gallery's two
Dashboard captures were regenerated once more since the streak label's
colour changed in the dark theme too.

**2026-08-15 — Friends system, phase 2 of 3 (QR generation in `settings.vue`).**
Follow-up to phase 1 (below), same session. Added a "My QR code" card to
`settings.vue` that renders a QR of the user's own `id` using the `qrcode`
npm package's `toString(id, { type: 'svg' })` — string-only SVG generation
(no canvas/`document` access), chosen specifically so it works identically
during SSR and renders in the initial server payload rather than popping in
after hydration. No backend change needed: the id was already available
client-side from the session. See ADR-0017's new "QR generated as an inline
SVG string" subsection for the full rationale, including why `v-html` here
doesn't carry the XSS risk the lint rule normally flags (the SVG is
generated locally from the account's own UUID, never from anything
user-controllable) — a scoped `eslint-disable-next-line` with that
justification was added rather than suppressing the rule globally.

Verification went beyond visual: rather than trust the rendered QR "looks
right," the actual `<path>` `d` attribute in the live DOM (the data that
encodes the QR's module grid) was extracted via Playwright and compared
byte-for-byte against an SVG independently regenerated server-side with the
same library/options for that session's real user id — an exact match,
proving the page encodes the correct id rather than merely rendering *a*
QR-shaped image. `npm run lint`/`typecheck`/`build` all clean; i18n keys
(`settings.qr.*`) added to all three locales with parity re-checked (419
keys each). The QR is real but currently unscannable by anything in the
app — phase 3 (Android) is what adds a scanner; this is called out
explicitly in ADR-0017's Consequences so it isn't mistaken for a working
end-to-end flow.

**2026-08-15 — Friends system, phase 1 of 3 (backend + web, no QR yet).**
The user asked for a friends feature: add someone by username, or by
scanning a QR shown on their profile. Stage 9's friends item
(`docs/roadmap/TASKS.md`) had been undesigned since tournaments (ADR-0016)
was pulled forward ahead of it. Given the size (backend + web + a brand-new
Android profile screen for the QR-scanning half), this went through
`EnterPlanMode` first: two research agents (current `users`/`playgroups`
schema and DTO shapes; existing invite/QR precedent — `playgroups`' username
search, `tournaments.join_code`, and confirming no QR library exists
anywhere in the repo yet) fed a written plan, approved before any code was
touched. The plan split the work into 3 phases and only phase 1 (backend +
web, request lifecycle via username search) was built this session; phases
2 (web QR generation) and 3 (Android profile screen + QR generation/scanning)
are follow-up work. Full design — most notably, why the QR encodes the
user's own `id` directly instead of a new rotable `friend_code` — is in
[ADR-0017](../decisions/0017-friends-system.md); this entry covers the
how/verification.

- **Schema**: migration `00017_friend_requests.sql`, one table
  (`friend_requests`, `requester_id`/`addressee_id`/`status`), RLS enabled
  (deny-all, same as every table since the 2026-08-01 audit). No separate
  `friends` table — an `accepted` row *is* the friendship.
  `docs/database/schema.dbml` updated in the same pass; `dbml2sql` and
  Spectral (`openapi.yaml`) both run locally to confirm the sources of
  truth stay valid (0 errors; the openapi lint's 154 warnings are all
  pre-existing `operationId`/`tags`/`description` gaps across the *entire*
  file, present before this change too, confirmed by diffing).
- **Backend**: new module `internal/friends` (handler/service/query.sql,
  same layout as `internal/playgroups`), wired in `cmd/api/main.go`. Reuses
  `GET /users/search` unchanged for the username-search entry point. Handles
  the crossed-request race (A and B request each other before either
  responds) by auto-accepting the existing reverse row instead of erroring
  or double-inserting — see ADR-0017's Decision section.
- **Verification, unusually thorough for this sandbox**: neither Docker nor
  a live Postgres was assumed available, so both were set up from scratch —
  `sqlc` v1.27.0 (pinned, matching CI, after an initial `@latest` install
  produced a version-string-only diff that was reverted) generated via a
  real `go install`, and a local `postgresql-16` package (already installed
  in the image but not running) started, password set, `pg_hba.conf`
  switched to `md5`, and `commander`/`commander_test` databases created by
  hand. `goose ... up` applied all 17 migrations cleanly against
  `commander_test`, then `go test -race -p 1 ./...` ran the **entire**
  backend suite (not just the new package) against it — every existing
  package still green, confirming nothing was broken. Beyond that: the
  actual API was run (`go run ./cmd/api`) alongside a real `nuxt dev` web
  client pointed at it, and a Playwright script (installed standalone in
  the scratchpad, pointed at the sandbox's pre-fetched Chromium binary since
  the web project itself has no Playwright dependency) drove two real
  browser sessions through the full UI: register both users, search-and-send
  from A, accept from B, and confirm both ended up seeing each other in
  `/friends` with the pending entries cleared — screenshots captured at each
  step. `npm run lint`/`typecheck`/`build` (real `nuxt build`, not just
  typecheck) all clean; the one lint finding (`apiFetch<void>` invalid as a
  generic argument on the DELETE/reject calls) was fixed to `apiFetch<null>`,
  matching `useSettings.ts`'s existing convention for a 204-with-no-body call.
- **i18n**: `friends.*`/`toast.friend*`/`errors.friends.*` keys added to all
  three locales (`en`/`es`/`ca`), not just `en` — key-parity checked
  programmatically (417 keys in each file, zero missing on either side)
  rather than assumed.

**2026-08-09 — Branch migration off an unrelated in-flight branch;
dependency updates (Android + web, last item of the punch list).**
Mid-session, `git branch --show-current` turned up something unexpected:
the working checkout was on `claude/web-tracker-turn-pause`, not `main` —
and that branch already carried 2 commits neither the user nor this
session had made ("Add turn passing and pause/resume to the web life
tracker", "Add reset-lives pause action and orbiting turn label to the
Android tracker"), already pushed to `origin`. Flagged to the user
immediately rather than silently committing on top of someone else's
in-flight work. `origin/main` turned out to already have that same work
merged (PR #80, squashed as `7bc69c3`), which simplified the fix: stashed
every uncommitted change from this session (reaper, `/playgroups`
pagination, tournaments — see the entries above), fast-forwarded `main` to
`origin/main`, branched `session-0908` from there, and popped the stash —
applied with **zero conflicts**, including in the three `web/i18n/locales/*.json`
files both this session and PR #80 had independently touched. Verified
clean on the new base: backend `go build`/`go vet`/`golangci-lint` (same
pre-existing CRLF-only findings as before), and web `eslint`/`nuxt
typecheck` both 0 issues.

With that resolved, closed out the punch list's remaining items:
- **`schema.dbml` drift** (open since 2026-08-01): found already fixed on
  `main` — see the Stage 2 entry below.
- **Dependency updates**: web via `npm update` (respects the existing `^`
  ranges, so TypeScript's major jump stayed excluded automatically),
  verified with a real `npm run build`, not just lint/typecheck — a
  meaningfully stronger check than the two used until now, since a
  production build catches issues that pass lint. Android via
  `libs.versions.toml`, applied with the user explicitly choosing "the
  whole set, unverified" after being told the risk — then, later the same
  session, actually verified locally with a real `./gradlew build` (JDK 21
  + Android SDK are present in this environment after all; the earlier
  "no toolchain" belief was wrong). The one flagged risk — whether KSP
  `2.3.11` supports Kotlin `2.4.10` given KSP dropped its old
  `<kotlin>-<ksp>` versioning scheme — turned out to be a non-issue,
  `kspDebugKotlin`/`kspReleaseKotlin` ran clean. Two unrelated real
  breakages surfaced instead, both fixed and pushed (commit `d060709`):
  AGP 9.3.1 hard-errors on the deprecated `kotlinOptions { jvmTarget = "1.8" }`
  setter (migrated to the `compilerOptions` DSL), and `navigation-compose`
  `2.9.8` removed `ExperimentalSafeArgsApi` entirely now that type-safe nav
  is stable (dropped the dead `@OptIn` in `AppNavigation.kt`), which in turn
  required raising the project's JVM target from 1.8 to 11 since several
  bumped androidx/Compose libraries now ship JVM 11 bytecode (safe, `minSdk
  26` desugars fine). `./gradlew build` now passes clean: compile, unit
  tests, lint, `assembleDebug`, `assembleRelease`.

**2026-08-05 — Statistics: opponent head-to-head, per-playgroup counts,
finished-games history; username availability check; TCP_NODELAY fix.**
Three independent pieces of work landed in this session:

- *Statistics.* Three new live-computed queries (`internal/statistics`,
  `query.sql`/`service.go`): `ListOpponentStats` (head-to-head record —
  games together, times each side eliminated the other — against every
  opponent the user has shared a finished game with),
  `ListPlaygroupGameCountsForUser` (every playgroup the user belongs to with
  its finished-games count in one query, replacing the web client's old
  per-group loop over `GetPlaygroupStats`), and `ListFinishedGamesPage`
  (keyset pagination over `(created_at, id) DESC`, same shape as
  `games.ListGamesPage` but owned separately in `statistics` because it
  needs denormalized player/deck/username data the shared `GET /games`
  deliberately doesn't carry). Exposed as `GET /statistics/opponents`,
  `GET /statistics/playgroups`, `GET /statistics/games`
  (`docs/api/openapi.yaml` updated). Web `statistics.vue` gained a
  decks/games tab split, deck sorting (recent/win rate/games played), and
  "most played group"/"archenemy" summary cards built from the new
  endpoints. Android got a matching `FinishedGamesViewModel` plus
  `StatisticsScreen`/`StatisticsViewModel` updates.
- *Username availability.* `GET /users/username-available?username=`
  (`internal/users`): public, unauthenticated, rate-limited alongside the
  other auth endpoints (an easy enumeration target otherwise), exact
  case-sensitive match against the same constraint `UpdateUsername` writes
  under. Wired into the web registration form (debounced check as the user
  types, submit blocked client-side if the name is already taken — the
  server-side check on actual submission is still the real guard).
- *Perf: TCP_NODELAY.* Found while investigating request latency: fasthttp
  (Fiber's engine) never disables Nagle's algorithm on accepted connections,
  so a keep-alive connection reused across requests stalled ~40ms per
  request (Nagle plus the peer's delayed ACK) waiting to see a header/body
  write split across two TCP segments — confirmed locally, ~45ms vs. ~5-7ms
  for the same POST over a fresh connection. Fixed with a small
  `net.Listener` wrapper (`cmd/api/listener.go`) that sets
  `SetNoDelay(true)` on every accepted `*net.TCPConn`, plugged into
  `main.go` via `app.Listener(...)` instead of `app.Listen(...)`.

**2026-08-01 — Documentation restructuring.** Explicit user request, after a
full project review that also flagged two code issues (see the Stage 1
entries on `payload.amount` validation and the `FinishGame` double-counting
race). Split TASKS.md's "Last reviewed" field — a single ~30KB line
chaining "Previous review, same day: ..." entries going back to
2026-07-26 — into this file. TASKS.md keeps one line per item; this file
keeps the narrative. Nothing was deleted, only moved and, where it was
pure repetition across three or four passes over the same paragraph,
trimmed.

**2026-08-01 — Security audit follow-up #3** ("los headers de seguridad del
cliente web, dale"): closed the last open Low finding from the same-day
audit — the web client sent no `Content-Security-Policy`/
`Strict-Transport-Security`/`X-Frame-Options`/`X-Content-Type-Options`/
`Referrer-Policy`. New `web/server/plugins/security-headers.ts` +
`web/server/utils/security-headers.ts` set all five on every response,
gated to non-dev only (`!import.meta.dev` — Vite's dev client needs
eval-based HMR and a same-origin WebSocket this CSP would block, and dev is
never internet-facing). The one real design decision: `script-src` isn't
`'unsafe-inline'` (that would defeat the point of adding a CSP at all) —
Nuxt always renders at least one inline `<script>` (the
`window.__NUXT__=...` runtime-config payload, sometimes a Vite
`type="importmap"` one too), so a first attempt at nonce-based CSP via the
`render:html` hook was tried and empirically broke hydration (the
importmap script isn't reachable through that hook, confirmed via real
console CSP-violation errors). Switched to hashing every inline script's
actual content from the final composed HTML body (`beforeResponse` hook)
and allowlisting those exact hashes — verified end-to-end with a headless
Chromium session against a real `npm run build` + `node
.output/server/index.mjs`: no CSP console errors, real hydration (typed
into the login form, read the value back), working SPA navigation.
`style-src` keeps `'unsafe-inline'` (the app uses `:style="..."` bindings
in several places, CSP has no hash/nonce mechanism for those) — documented
as a deliberate scope limit in `web/README.md`. This closed the audit
entirely except two Low items left open on purpose (rate limiter behind an
unconfigured `ProxyHeader`, the `cc_session` marker-cookie tradeoff — both
already documented in-repo as accepted).

**2026-08-01 — Security audit follow-up #2** ("acabemos con el refresco e
[sic] token por robo"): closed the last open Medium finding — refresh
token rotation (`internal/auth/service.go: Refresh`, see
[ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)) had no
theft/reuse detection. Reusing an already-revoked refresh token (rotated
away by an earlier `Refresh`, or killed by `Logout`) is now treated as a
signal of theft: `Refresh` revokes *every* active refresh token for that
user (`RevokeAllRefreshTokensForUser`) instead of just rejecting the reused
one, so a stolen token replayed after the legitimate client already
rotated past it can't keep regenerating a session indefinitely — the
account owner is logged out everywhere and has to log in again, same as
standard OAuth2 refresh-rotation. New regression test
`TestRefresh_ReuseOfRotatedToken_RevokesTheWholeSessionFamily`.

**2026-08-01 — Security audit follow-up #1** ("avanza con el 1, 2 y 4" over
the Medium findings list): closed 3 gaps. (1) `statistics.GetPlaygroupStats`
had no membership check — any authenticated user with a playgroup UUID
could see its aggregated stats; `statistics.Service` now takes a
`PlaygroupMembership` dependency, denies with `ErrPlaygroupNotFound`
(no-reveal) if the caller isn't a member. (2) `games.CreateGame` never
validated the caller belonged to `playgroup_id` — any user could create
games "belonging" to a group they had no relation to; now requires
membership too. (3) `GET /users/search` had no rate limit of its own —
added `newSearchRateLimiter` (20 req/min/IP, `cmd/api/main.go`);
`openapi.yaml` gained the `429` response. New regression tests:
`TestGetPlaygroupStats_NotAMember_ReturnsNotFound`,
`TestCreateGame_NotAMemberOfPlaygroup_ReturnsNotFound`. The `/auth/register`
409 enumeration flagged in the same audit was left as-is (already covered
by the auth rate limit; revealing "email taken" is normal registration UX).

**2026-08-01 — Security audit + fixes** ("soluciona los críticos y altos"):
closed two Critical, pre-existing authorization gaps, unrelated to any
specific feature request. `GetGame`/`StartGame`/`FinishGame`/`GetTimeline`
never checked the caller had any relationship to the game — any
authenticated user could read, forcibly start, or forcibly finish ANY game
in the system, corrupting its statistics and broadcasting a fake
`game_finished` event to real participants; and the global `GET /games`
(no `playgroup_id`) returned every game in the database with no per-user
filter. Fixed: `internal/games/service.go` gained `authorizeGameAccess`
(playgroup membership if the game has one, otherwise a seat in it, same
trust boundary as ADR-0013's proxy-join), `ListGamesPage` now filters by
`user_id` via `EXISTS (... game_players ...)`, and the WebSocket handler
gained a `MembershipChecker` (`games.Service.CanAccessGame`) so a
connection is rejected before being registered in the Hub if the caller
can't access that game. New regression tests in
`internal/games/service_test.go`. Also fixed, same pass: the web BFF's
`/api/backend/[...path]` blocked `auth/*` with a case-sensitive
`startsWith`, bypassable via `/api/backend/Auth/login` since Fiber's router
is case-insensitive by default — now lowercases the path first. And the
backend now fails fast at boot (`internal/config/config.go`) instead of
silently falling back to the publicly-known default `JWT_SECRET` or an
open `CORS_ALLOWED_ORIGINS=*` when `APP_ENV=production` and those aren't
explicitly set — previously a forgotten env var in a real deploy meant a
total auth bypass with a secret visible in this repo's own git history.
The remaining Medium/Low findings (ownership check on `CreateGame`, no
refresh-token-reuse detection, missing security headers) were intentionally
left open for that pass — see the three follow-ups above, all since closed.

**2026-08-01 — Android: manual language switcher, domain layer, WebSocket
client, live reconciliation, statistics UI, second-device join.** Several
same-day passes, all explicit user requests, all landed without a
`./gradlew` run in this environment (the sandbox's network policy blocks
`dl.google.com`, so the Android Gradle Plugin can't be resolved here — each
was reviewed by hand against existing patterns instead; each needs a real
`assembleDebug lintDebug testDebugUnitTest` run in an environment with
Google Maven access before merging). Full detail under Stage 4/5/6/7 below.
Also this day: `web/vercel.json` gained an `ignoreCommand` so an
Android-only PR doesn't trigger a no-op Vercel preview deploy (see
[ADR-0015](../decisions/0015-deployment-infrastructure.md)); and
housekeeping — confirmed `.github/modernize/java-upgrade` as scaffolding
unrelated to this project's stack (removed where present) and trimmed
duplicated "4 sources of truth"/layer-pattern narrative that
`README.md`/`ARCHITECTURE.md`/`ROADMAP.md` had all been independently
restating (`ARCHITECTURE.md` is now the single canonical location).

**2026-08-01 — Real fullscreen for the web life tracker** (user request):
`app/pages/play.vue` already filled the browser window via CSS but never
requested real browser fullscreen. New `useFullscreen.ts`
(`isFullscreen`/`isSupported`, `toggleFullscreen()` via
`requestFullscreen()`/`exitFullscreen()`, synced with the native
`fullscreenchange` event) and a toggle button next to the existing `✕`.

**2026-07-30 — `main` sync after merging PRs #48/#49/#50.** Audited 3 real
changes against TASKS.md, which mentioned none of them yet.
- **Backend (#48)**: the `api` binary now applies embedded goose migrations
  on startup (`internal/common/migrate.go`), because Render's free tier
  offers no "pre-deploy/release command" hook — migrations were never
  actually being applied in production against Supabase before this.
  `backend/Dockerfile` now copies `migrations/` into the final image.
  `backend/README.md` gained a "Deployment (Render)" section (env vars,
  Supabase Session pooler — not Transaction pooler, the backend uses
  prepared statements via pgx) and a documented limitation: with >1
  replica, all of them would run `goose up` in parallel on boot (goose is
  idempotent but the runs aren't serialized with a lock) — harmless today,
  single-instance deployment.
- **Web (#49)**: full internationalization with `@nuxtjs/i18n` — see
  [ADR-0014](../decisions/0014-web-internationalization.md) (single active
  locale `es`, `strategy: 'no_prefix'`, ~200-250 keys, and along the way
  fixes the copy from Argentine "voseo" to Spain's "tuteo"). Same PR: the
  Google Sign-In button moves to `pill` shape with a theme-dependent style;
  Nuxt gained a default `<title>`/`titleTemplate` (previously none — a
  WCAG 2.4.2 violation flagged by axe/Lighthouse).
- **Android (#50)**: full visual redesign (violet/dark palette, new
  `Gradients.kt`, new shared `AppComponents.kt` replacing `PlayerCard.kt`).
  `PreGameScreen`/`GameTrackerScreen` now require landscape orientation
  with a new seat grid, pause overlay, and final game summary — none
  existed before. **Functional, not just visual**: the poison counter goes
  from unmodeled to real (`PlayerState.poison`,
  `PlayerState.isEliminated()` now centralizes all 3 elimination
  conditions, previously only 2 lived in a private `isAlive`). New
  registration flow (`RegisterScreen`/`RegisterViewModel`) — didn't exist
  on Android before, only on web.

**2026-07-28 — Documentation audit** ("check whether we've left any
documentation out of date"): 3 real gaps found, all for code already
committed earlier the same day but never reflected in TASKS.md — the
Playgroup screens on the web client, the Settings screen (password change
+ Moxfield linking + bulk import trigger), and a stale description of
`JoinGame` that predated the proxy-join work (ADR-0013). `openapi.yaml` and
the ADRs were already current; the gap was specific to TASKS.md.

**2026-07-28 — `openapi.yaml` audit against real routes**: ~39 routes
across 10 modules enumerated and compared one by one against the spec. 3
real discrepancies fixed: `GET /games/{id}` was undocumented (added);
`GET /users/{id}` was documented but has no handler (removed from spec);
`PATCH /decks/{id}` was documented but doesn't exist (removed). The
remaining ~36 paths verified field-by-field against their Go DTOs with no
further divergence found.

**2026-07-28 — Search users, proxy-join, Casual/Group mode.** Same day, 3
related passes. `GET /users/search?q=` closed a literal TODO in the web's
"add member" form ("ask your friend for their user ID"); found and fixed a
real bug along the way — no results returned JSON `null` instead of `[]`,
breaking any JS/TS client that assumes an array. Backend gained
`game_players.added_by` + proxy-join (`POST /games/{id}/join` with
`user_id`, gated by shared playgroup membership) — see
[ADR-0013](../decisions/0013-proxy-join-and-action-authorization.md) —
and closed, in the same change, a pre-existing gap where
`POST /games/{id}/actions` never validated that `actor_id` belonged to the
caller at all. Android's `PlayerSetupScreen` gained a Casual/Group
selector on top of that: Group mode lets you pick a real playgroup and
assign members (with their decks) to seats, seating them via proxy-join in
one bootstrap call; this superseded an explicit "that's me" chip + own-deck
picker that had been added hours earlier the same day (kept in the log
below as a superseded step, not deleted).

**2026-07-27 — Cross-cutting infra cleanup**: confirmed `docker/`/`scripts/`
never existed (no git history for them, tree diagram fixed); added
`.github/dependabot.yml` (4 ecosystems, weekly); found and fixed a real CI
gap — `backend-ci.yml`/`android-ci.yml`/`web-ci.yml`'s path filters also
matched each directory's own README, so a README-only change triggered the
full heavy battery (for backend: real Postgres + migrations + lint). Fixed
with `predicate-quantifier: 'every'` + `!*/README.md` exclusions.

**2026-07-27 — Full documentation audit**, centralizing the root
`README.md` as the index of every doc in the repo (previously linked only
3). Main finding: the Android UX/navigation docs had been written in a
parallel worktree at the same commit that connected Android to the real
backend, and were left describing the pre-connection app. All corrected.
Also corrected: several ADRs that still described superseded states
(ADR-0004 said Moxfield/statistics were pending when they were done;
ADR-0009 said "no repository layer" when one existed; ADR-0010 said
`internal/websocket/` was empty when it wasn't), a stale README claim that
`web-ci.yml` didn't exist, and several TASKS.md items that were already
done but still unchecked.

**2026-07-27 — PostgreSQL 18, deck image, docker-compose consolidation,
Stage 0 closed, initial Android↔backend connection.** PostgreSQL pinned to
18 everywhere (previously misaligned 15/16 between compose and CI; the
volume mount layout also changed in 18+, `/var/lib/postgresql` instead of
`/var/lib/postgresql/data`). Imported decks now carry `image_url` (art crop
from Moxfield's own `main.id` field, migration `00006`). `docker-compose.yml`
centralized at the repo root with a real `web/Dockerfile`, fixing two bugs
found while bringing up the full stack: `api` not waiting for Postgres to
be *healthy*, and SSR calls from the `web` container unable to resolve
`localhost:8080` (split `NUXT_API_BASE` internal vs. `NUXT_PUBLIC_API_BASE`
browser-facing). `backend/README.md` created. Stage 0's 4 pending
deliverables (use cases, wireframes, diagrams, foundational ADRs) closed
the same day. Android connected to the real auth backend for the first
time (`LoginViewModel`, `NetworkModule`, `SessionManager`,
`AuthInterceptor`/`AuthAuthenticator`) — only the external Google Cloud Web
Client ID step remained. Stage 2 (database) closed: explicit indexes,
CHECK constraints, exported ER diagram. Initial Nuxt web client skeleton
scaffolded (email/password + Google login). Stage 6 WebSocket: protocol
design ([ADR-0005](../decisions/0005-websocket-protocol.md)) and real
server implementation (`internal/websocket/`) both landed.

**2026-07-26 — Initial audit and backend build-out**: auth (Google OAuth
included), generated the `playgroups`/`games`/`game-actions` slices,
sqlc/lint tooling fixes, GitHub Actions quality gates, repo linked with
branch protection on `main`, real email/password + Google auth, CORS, the
manual `tools/auth-test/` tool, real `decks`/Moxfield wiring, first backend
integration tests, real `games`/`game-actions` (the game engine itself),
real `playgroups`/`statistics` wiring (recalculation on game finish) — by
the end of this day, **the backend was 100% connected to the real
database, no dummy modules left**. Also: the complete Android local life
tracker with Room persistence, and `PreGameScreen` with the turn
randomizer and mulligan tracking.

---

## Per-stage narrative

### Stage 1 — `payload.amount` validation (found and fixed 2026-08-01)

`internal/game-actions/service.go`'s `payloadAmount` only checked that
`payload.amount` existed and was numeric — no sign check, no range check.
Reproduced against a real Postgres instance before fixing:

- `CombatDamage` with `amount: -500` was accepted and *healed* the target
  (life went up, and the negative amount was added to
  `total_damage_dealt`).
- `CombatDamage` with `amount: 3e9` was accepted: `int32(float64(3e9))`
  doesn't error, it silently wraps to a large *negative* int32 — the
  target's `life_total` became `-2147483608` (instant elimination, but for
  the wrong reason and with a nonsensical stored value).
- `PoisonCounter` with `amount: -50` was accepted, producing
  `poison_counters = -50`.
- `CommanderDamage` with a negative amount was the one case already caught
  — by the database's own `CHECK (amount >= 0)` on `commander_damage`,
  surfacing as an ugly `500` instead of a clean `400`.

Fixed: `CombatDamage`/`CommanderDamage` now go through
`payloadPositiveAmount` (rejects `<= 0`) since their `amount` is a
quantity of damage, never a signed delta; `LifeChange`/`PoisonCounter` keep
accepting negative values on purpose (a signed delta, e.g. to correct a
misclick) but every `payload.amount` — regardless of action type — is now
bounded to a whole number with magnitude ≤ 1,000,000
(`maxAmountMagnitude`, `payloadAmount`), rejecting fractional values,
`NaN`, and anything that would risk wrapping past `int32`'s range. New
errors: `ErrAmountOutOfRange`, `ErrAmountMustBePositive`. Regression tests
added in `internal/game-actions/service_test.go`:
`TestRecordAction_DamageAmount_MustBePositive`,
`TestRecordAction_AmountOutOfRange_ReturnsBadRequest`,
`TestRecordAction_NonIntegerAmount_ReturnsBadRequest`,
`TestRecordAction_PoisonCounter_NegativeAmountStillAllowed` (locks in that
poison correction still works, since it's deliberately not restricted the
same way as damage). `openapi.yaml`'s `/games/{id}/actions` payload
description and 400 response updated to match. Full backend test suite
(`go test -race -p 1 ./...`) and `golangci-lint` verified clean after the
change.

### Stage 1 — `FinishGame` concurrency / statistics double-counting (found and fixed 2026-08-01)

Two issues that compound:

1. `games/service.go: FinishGame` reads `game.Status != statusActive`
   outside any transaction, several round-trips before the actual
   `UPDATE`. The SQL itself
   (`internal/games/query.sql: FinishGame`) has no `AND status = 'active'`
   guard — it's `UPDATE games SET status = 'finished', finished_at = now()
   WHERE id = $1`, full stop. Two concurrent `FinishGame` calls on the same
   game can both pass the check and both succeed.
2. `statistics.RecalculateForGame` is named like a recompute, but it's
   purely additive (`ON CONFLICT DO UPDATE SET games_played =
   user_statistics_summary.games_played + EXCLUDED.games_played`, same
   pattern for every other counter). There's no path to recompute a user's
   or deck's stats from scratch — once inflated by a duplicate run, it
   stays inflated.

Reproduced with a Go test issuing 8 concurrent `FinishGame` calls (from
both seated players' callers) against one real 2-player game over
Postgres: all 8 succeeded, and both players' `games_played`/derived
counters ended up multiplied by 8x. With 2 concurrent calls it's
racier (often only 1 succeeds), but it does happen. The realistic trigger
isn't exotic: two players tapping "Finish" at the same time in Group mode,
or a client retrying after a timeout.

This directly contradicts `schema.dbml`'s own note on
`user_statistics_summary`/`deck_statistics_summary`: "Pre-calculated
statistics (recalculated if a historical game is modified)" — that's not
actually possible today. See [ADR-0011](../decisions/0011-migration-strategy-and-statistics-recalculation.md),
which already flags "`RecalculateForGame` is incremental, not idempotent —
each game must be processed exactly once" as an invariant to respect, but
nothing currently enforces it.

**Fixed the same day.** `internal/games/query.sql`'s `FinishGame`/`StartGame`
queries now guard their `UPDATE` with the status they require
(`AND status = 'active'`/`'pending'`) instead of relying solely on the
service's earlier read-then-write check — that check is kept as a
fast-path (avoids the write entirely for an obviously-wrong-state game),
but the guard on the `UPDATE` itself is what actually makes the state
transition atomic. `internal/games/service.go` now treats `pgx.ErrNoRows`
from either call as the corresponding existing domain error
(`ErrGameNotActive`/`ErrGameAlreadyStarted`, both already mapped to `409`)
instead of wrapping it as an unexpected error. `sqlc generate` re-run,
diff reviewed (only the two queries' `WHERE` clauses changed, nothing
else). New regression test,
`TestFinishGame_Concurrent_OnlyOneSucceedsAndStatsAreNotDoubleCounted`
(`internal/games/service_test.go`): fires 8 concurrent `FinishGame` calls
(alternating between both seated players) against one real Postgres-backed
game, asserts exactly one succeeds and the rest get `409`, and asserts
both players' recalculated `games_played` is exactly 1 — run 5x with
`-race` with no failures. Full backend suite (`go test -race -p 1 ./...`,
16 packages) and `golangci-lint` verified clean after the change.

This closes the immediate bug (double-counting under a race) but doesn't
add a general recompute-from-scratch capability: `RecalculateForGame` is
still purely additive by design. That's now provably safe, since the guard
above guarantees it's only ever invoked once per game's finish transition
— but it still means there's no way to *re-derive* `user_statistics_summary`/
`deck_statistics_summary` if the aggregation formula itself ever changes,
which is exactly the gap [ADR-0011](../decisions/0011-migration-strategy-and-statistics-recalculation.md)
already flagged and proposed a (never-implemented) `recalculate-stats`
command for. Left as a separate, larger piece of future work.

### Stage 4/5 — `SessionManager.refreshAccessToken()` concurrency (found and fixed 2026-08-01)

`AuthAuthenticator.authenticate()` calls `runBlocking { sessionManager.refreshAccessToken() }`
synchronously, on whichever OkHttp dispatcher thread received the 401 — and
OkHttp genuinely runs concurrent requests on separate threads, so this is a
real multi-threaded call site, not just a theoretical one. The scenario
that makes it likely rather than exotic: `LoadStatisticsUseCase` fires a
batch of `async`/`awaitAll` calls (global stats, then every deck's and
every playgroup's stats) that all share the same access token; once that
token expires, opening the Statistics screen can 401 several requests at
once. Before this fix, `refreshAccessToken()` had no synchronization at
all — each concurrent caller independently read the stored refresh token,
called `POST /auth/refresh`, and saved the response. Since the backend
**rotates** the refresh token on every use (see
[ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)) and, as
of the 2026-08-01 backend security-audit pass above, treats presenting an
already-rotated-away refresh token as a signal of theft — revoking the
*entire* session family, logging the user out on every device — two
concurrent refresh calls racing against each other could trigger exactly
that: the loser presents a token the winner already rotated away,
`SessionManager.forceLogout()` fires, and the user is logged out
everywhere over what was really just a timing coincidence between two of
their own requests.

**Fixed the same day** with a new `core/util/SingleFlight.kt`: a small,
Android/Context-free class that collapses concurrent callers of a suspend
block into a single real execution, sharing the one result — the same
shape as the web client's own `inFlightRefresh` map
(`web/server/utils/backend.ts`), which already solved the identical
problem in the Nitro BFF (see that file's own doc comment, which
independently arrived at the same fix for the same underlying reason).
`SessionManager` gained a class-level `refreshScope`
(`CoroutineScope(SupervisorJob() + Dispatchers.IO)`, deliberately NOT tied
to any individual `runBlocking` call's own scope — it needs to outlive
every one of them for the dedup to work across separate OkHttp threads)
and now runs `refreshAccessToken()`'s body through
`refreshSingleFlight.run { ... }`.

`SessionManager` itself remains untestable in a pure-JVM test (the
pre-existing, documented gap: it has a `Context` constructor parameter, no
Robolectric in this project) so the fix couldn't get a `SessionManagerTest`
the normal way — same reasoning already on file for
`SettingsViewModel`/`DashboardViewModel`/etc. Followed the project's own
established workaround instead (see `GameSocketReconnect.kt`'s doc: "kept
as a plain function... so the retry/backoff behavior can be unit tested...
without a real socket"): `SingleFlight` is a standalone, dependency-free
class with its own test, `SingleFlightTest.kt` — 3 cases: N concurrent
callers collapse into exactly one execution of the block and all receive
its result; a call made after the previous one completed triggers a fresh
execution (not permanently cached); and a failing block's exception
propagates to the caller while still clearing the in-flight state so the
next call retries instead of getting stuck.

**Verification note**: this sandbox's network policy still blocks
`dl.google.com`, so `./gradlew` couldn't run here, same limitation as
every other Android pass in this log. Unlike those previous passes, this
one wasn't only reviewed by hand — the risk was concentrated entirely in
`SingleFlight.kt`'s coroutine logic, which has zero Android/Hilt/Room
dependencies, so it was possible to fetch a standalone Kotlin compiler
(2.2.10, matching `libs.versions.toml`) and `kotlinx-coroutines-core`/
`-test` straight from Maven Central (reachable even though
`dl.google.com` isn't), compile `SingleFlight.kt` + `SingleFlightTest.kt`
outside Gradle entirely, and run the tests with real JUnit — all 3 passed.
The exact `return@run` early-return idiom used in `SessionManager`'s new
`refreshAccessToken()` body (`someSingleFlightInstance.run { ... return@run
null ... }`) was independently compiled and executed in isolation too, to
confirm Kotlin resolves the label to `SingleFlight.run`'s lambda and not
something else. Still needs a real `assembleDebug lintDebug
testDebugUnitTest` run in an environment with Google Maven access before
merging, same as every other Android change in this log — this only
closes the gap for the specific new logic, not for the full app.

### Stage 2 — `schema.dbml` drift from real migrations (found 2026-08-01, found already fixed 2026-08-09)

Verified by compiling `schema.dbml` to SQL (`dbml2sql`), loading it into a
throwaway Postgres database, and diffing `information_schema.columns`
against a second database with the real goose migrations applied. Result:
the entire `deck_resync_jobs` table (added by migration
`00013_deck_resync_jobs.sql`, part of Stage 8's "resync all decks" feature)
is missing from `schema.dbml`. No other drift found — the rest of the
schema matches exactly, and `docs/api/openapi.yaml` is independently
correct against the real registered routes (verified separately).

This matters beyond one missing table: `docs-ci.yml`'s `dbml-validate` job
only checks that `schema.dbml` compiles to valid SQL, not that it matches
the actual database — so a migration can land without its DBML counterpart
and CI stays green indefinitely, in direct contradiction of README.md §3's
rule to "edit `schema.dbml` first." Not fixed in the documentation-
restructuring pass it was found in (out of scope that session) — left as
an open TASKS.md item.

**Revisited 2026-08-09** (working through the user's prioritized punch
list): `schema.dbml` already had the complete, correct `deck_resync_jobs`
table — status CHECK, partial unique index, everything — by the time this
item came up again. Fixed in some pass between 2026-08-01 and now that
didn't check this TASKS.md box off. TASKS.md updated to reflect reality
instead of doing the (already-done) work over. The actual underlying gap
this item flagged — no CI check catches DBML/migration drift
automatically — is still real and still unaddressed; that part of the
item stays open.

### Stage 6 — No WebSocket heartbeat (found 2026-08-01, fixed 2026-08-10)

`internal/websocket/handler.go` set a read deadline only for the initial
auth handshake (`authMessageTimeout`, 10s), then cleared it entirely
(`SetReadDeadline(time.Time{})`) once authenticated. `client.go`'s
`readLoop` was then an unbounded `ReadMessage()` loop with no ping/pong
anywhere in the package. A client that disappears from the network without
a clean TCP close — the ordinary way a phone loses connectivity, not an
edge case for this app's primary use case (a life tracker meant to sit on
a table during a game) — left its `Hub` room entry and both of the
client's goroutines (`readLoop`, `writePump`) running until something else
tore down the whole game's room. [ADR-0005](../decisions/0005-websocket-protocol.md)
explicitly listed "application-level heartbeat" as out of scope for the
initial protocol design, so this wasn't an oversight relative to what was
planned — but it was worth revisiting given how often the described
disconnect actually happens in practice.

**Fixed 2026-08-10** (`internal/websocket/client.go`): `writePump` now
sends a WebSocket ping frame every `pingPeriod` (54s, via
`conn.WriteControl(fiberws.PingMessage, ...)`); `readLoop` sets an initial
`pongWait` (60s) read deadline and installs a pong handler that refreshes
it on every pong. If no pong arrives within `pongWait`, `ReadMessage`
returns a deadline-exceeded error, `readLoop` returns like any other read
failure, and `handleConnection`'s existing cleanup (`hub.Unregister` +
`client.Close()`) runs — same teardown path as a clean disconnect, just
now bounded in time instead of unbounded. `pongWait`/`pingPeriod`/`writeWait`
are per-`Client` fields (copied from package constants in `newClient`) so
tests can shrink them instead of waiting 60s real-time.

Chasing this down with `go test -race` also surfaced an unrelated
pre-existing race, fixed in the same pass: `writePump` used to be launched
as a bare `go client.writePump()` with nothing joining it. gofiber recycles
the `*fiberws.Conn` back to its `sync.Pool` the moment the connection
handler function (`handleConnection`) returns — but `writePump` runs in
its own goroutine and closing its channel only *signals* it to stop, it
doesn't block until it actually has. `handleConnection` could therefore
return (releasing `conn` to the pool for reuse by an unrelated future
connection) while `writePump` was still mid-`WriteMessage`/`WriteControl`
on that same `*Conn` — a real data race, not a theoretical one, caught
directly by `go test -race` once the new ping ticker made the race window
wide enough to hit reliably in a fast test loop. Fixed by adding
`Client.startWritePump()`/`Client.wait()` (backed by a `sync.WaitGroup`):
`handleConnection`'s deferred cleanup now calls `client.wait()` after
`client.Close()`, blocking until `writePump` has actually returned before
the handler function itself returns and gofiber reclaims `conn`.

**Verified**, all via Docker (no local Go toolchain in this sandbox):
`golang:1.25-alpine` ran `go build ./...`/`go vet ./...` clean across the
whole backend; `golangci-lint:v2.12.2` clean on `internal/websocket`
(zero new issues — the `mnd`/`noctx` findings the first draft introduced
were fixed by expressing `pingPeriod` as a plain duration literal instead
of a computed `pongWait * 9 / 10`, and by using `net.ListenConfig.Listen`
instead of the context-less `net.Listen` in the test helper; the
repo-wide `gofmt` CRLF noise from this Windows checkout is pre-existing
and unrelated). New regression tests in
`internal/websocket/client_internal_test.go` (package `websocket`, real
`*fiberws.Conn` over a loopback `net.Listener`, no fakes — `writePump`/
`readLoop` take the concrete type, not the `Hub`'s `Conn` interface, so a
fake wouldn't exercise the real code path): one client that connects and
never reads again (simulating the exact "phone loses signal" scenario)
must be reaped once the shrunk `pongWait` elapses; one client that keeps
its own read pump running (so it auto-answers pings with pongs, like any
real client) must **not** be disconnected across several ping periods.
Both run clean under `go test -race -count=10`.

### Stage 8 — Bulk Moxfield import and deck resync jobs can get stuck (documented when built, revisited 2026-08-01, fixed 2026-08-08)

Both `internal/moxfieldimport` and `internal/deckresync` run their import
loop in a bare goroutine with `context.Background()` (deliberate — the
request's context is cancelled the moment the handler returns) and their
own `recover()`. Neither had any mechanism to detect or retry a job left
`in_progress` by a process restart mid-run; the partial unique index that
allows only one active job per user then blocked starting a fresh one,
with no way out short of a manual DB fix. This was an accepted, documented
gap when both features were built ("Accepted, not solved, in this pass" in
the original code comments), and was worth revisiting given
[ADR-0015](../decisions/0015-deployment-infrastructure.md) puts the
backend on Render's free tier, which sleeps the service between requests —
exactly the kind of interruption this gap didn't handle.

**Fixed 2026-08-08** (prioritized first in a punch list the user asked for,
ranked most-to-least important, dependency bumps deliberately last): added
`ReapStaleJobs` to both packages (`moxfieldimport.ReapStaleJobs`,
`deckresync.ReapStaleJobs`), backed by two new sqlc queries
(`ReapStaleImportJobs`/`ReapStaleResyncJobs`) that `UPDATE ... SET status =
'failed', error_message = '<explanatory message>', finished_at = now() WHERE
status IN ('pending', 'in_progress')`. `cmd/api/main.go` calls both once at
startup, right after the DB pool connects and before the Fiber app starts
serving traffic (`reapStaleBackgroundJobs`), and logs how many rows each one
reaped. This is deliberately a blanket startup sweep, not a
staleness-timeout check on `updated_at`: in the current single-instance
deployment (see ADR-0010), whichever process is starting up is by
definition the only one that could ever have run these jobs, so anything
still `pending`/`in_progress` at that point cannot belong to it — it's
necessarily a leftover from a previous run that crashed or was restarted
mid-import/resync. Same caveat as the in-memory auth rate limiter: this
specific approach would need to change (to a real staleness timeout) if the
backend is ever deployed with more than one replica, since a job actually
in flight on a sibling replica would otherwise get reaped out from under it.
Regression tests `TestReapStaleJobs_MarksPendingAndInProgressAsFailed`
(`internal/moxfieldimport/service_test.go`) and
`TestReapStaleJobs_MarksInProgressAsFailed`
(`internal/deckresync/service_test.go`) create a job, force it into the
stuck status directly via the repo (bypassing the normal `StartImport`/
`StartResyncAll` flow, since those complete too fast to observe as stuck),
call `ReapStaleJobs`, and assert both that the row flips to `failed` with a
non-empty `error_message` and that the active-job unique index is freed —
a fresh `StartImport`/`StartResyncAll` for the same user succeeds
afterwards instead of 409ing.

This sandbox has no Go toolchain at all (not even `go version` resolves), so
`internal/moxfieldimport/query.sql.go`/`querier.go` and
`internal/deckresync/query.sql.go`/`querier.go` were initially hand-edited to
match sqlc's generated style instead of running `sqlc generate` directly.
Verified afterwards using the team's local Docker instead (per the user's
"usa el docker del equipo local"):
- `docker run ... sqlc/sqlc:1.27.0 generate` regenerated both packages'
  `query.sql.go`/`querier.go` — byte-for-byte identical to the hand-written
  version (confirmed via `git diff --numstat`), and also touched
  `internal/statistics` and `internal/users` with zero real content changes
  (pure CRLF/LF normalization noise from `core.autocrlf=true` on this
  Windows checkout, no `.gitattributes` pinning Go files to LF).
- `docker run ... golang:1.25-alpine sh -c "go build ./... && go vet ./..."`
  — both clean.
- `docker run ... golangci/golangci-lint:v2.12.2 golangci-lint run
  --timeout=5m` (same version pinned by `backend-ci.yml`) first caught three
  real issues from this change: `TestReapStaleJobs_MarksPendingAndInProgressAsFailed`
  exceeded the cyclomatic-complexity limit (`cyclop`, 11 > 10), and two
  `govet` `err`-shadowing warnings in `cmd/api/main.go` and the same test.
  Fixed by extracting `stuckImportJob`/`stuckResyncJob` and
  `assertFailedWithMessage` test helpers (bringing both tests' complexity
  well under the limit) and renaming the shadowed startup-error variable to
  `reapErr` (matching the existing `migrateErr` convention already used a
  few lines above it in `main.go`) and switching a `:=` to `=` in the test.
  A follow-up `lll` (line too long) hit on the extracted helper's signature
  was fixed by wrapping it. Re-running after fixes: 0 new issues — the only
  remaining lint output is the same 68 pre-existing `gofmt`/CRLF findings
  across the *entire* repo (every `.go` file, including ones untouched by
  this change), confirming that's a checkout artifact, not something this
  change introduced.
- A throwaway `postgres:18-alpine` container (own Docker network, not the
  project's `docker-compose.yml`) had `goose -dir migrations postgres ...
  up` applied against it, then `docker run ... golang:1.25-alpine sh -c "go
  test -race -cover -p 1 ./..."` (same flags as `backend-ci.yml`'s
  `build-test-migrate` job) passed for all 16 packages, including
  `internal/moxfieldimport` (75.8% coverage) and `internal/deckresync`
  (76.2% coverage). All throwaway containers/network/volumes were torn down
  afterwards.

### Stage 3 — Cursor pagination on `/playgroups`; `/games/{id}/timeline` deliberately left unpaginated (2026-08-08)

Second item of the same user-prioritized punch list as the Stage 8 reaper
above (paginate the remaining unpaginated listings, third on the list after
the timeline/resync reaper — WebSocket heartbeat and the alpha-exit email
verification flag were explicitly dropped from the list per the user's
follow-up instructions).

Before touching either endpoint, checked how the existing clients actually
consume them: Android's `CommanderApi.kt` declares both `getTimeline()` and
`listPlaygroups()` as returning a bare `List<...>` via Retrofit — switching
either response to `{items, next_cursor}` unconditionally would break
deserialization for every existing client build. Worse for the timeline
specifically: `GameViewModel.replayCommanderDamageUseCase` fetches the
*complete* timeline to reconstruct the per-seat commander-damage map for an
active game; a paginated fetch that only walked one page would silently
under-count damage instead of erroring, and a single game's action count is
bounded in a way that account-wide game/deck history isn't — there's no
real growth problem pagination is solving there. Given that risk/benefit,
this was raised with the user directly rather than assumed; they chose to
leave `/games/{id}/timeline` unpaginated entirely and only paginate
`/playgroups`.

**`GET /playgroups`** gained cursor pagination the same way `GET /games`
already handles `playgroup_id` — response shape branches on a query param
instead of changing unconditionally:
- New `Service.ListPlaygroupsPage(ctx, page, userID)` (`internal/playgroups/service.go`),
  backed by a new keyset query `ListPlaygroupsForUserPage` (`(created_at, id)
  DESC`, same shape as `games.ListGamesPage`/`decks.ListDecksPage`) — added
  to `query.sql`, with the generated `query.sql.go`/`querier.go` additions
  initially hand-written to match sqlc's style, then actually regenerated
  via `sqlc generate` in Docker (see the Stage 8 entry above for why this
  sandbox needs Docker for that) — which caught a real (if harmless) gap in
  the hand-written version: it was missing the `::uuid` cast on the
  `user_id` param that the `.sql` source's `sqlc.arg('user_id')::uuid`
  implies, unlike the moxfieldimport/deckresync reaper queries from the
  prior pass, which came out byte-identical. A reminder that "hand-write to
  match sqlc's style" is a stopgap for a sandbox without the toolchain, not
  a substitute for running the real generator before trusting the result.
- `Handler.ListPlaygroups` (`internal/playgroups/handler.go`) now checks for
  `cursor`/`limit` in the query string: neither present → unchanged
  behavior, the full membership list with `members` populated per group,
  exactly what existing clients already parse. Either present → a
  `PlaygroupListResponse` page (`{items, next_cursor}`), deliberately
  *without* `members` populated (a paginated listing is for browsing many
  groups, not showing every one's roster up front — fetch the detail via
  `GET /playgroups/{id}` for that).
- `docs/api/openapi.yaml`: `GET /playgroups` documents both response shapes
  via `oneOf` (array vs. `PlaygroupListResponse`), plus the new
  `PlaygroupListResponse` schema and the shared `CursorParam`/`LimitParam`.
  Verified with `docker run ... node:22-alpine npx @stoplight/spectral-cli
  lint docs/api/openapi.yaml --ruleset .spectral.yaml`: 0 errors, the same
  ~116 pre-existing warnings (missing `operationId`/`tags`/`description`
  across the whole spec, already documented as non-blocking) — the new
  `/playgroups` `oneOf` schema didn't add any new warnings.
- Regression tests in `internal/playgroups/service_test.go`:
  `TestListPlaygroupsPage_OnlyReturnsMemberships` (same membership scoping
  as the unpaginated `ListPlaygroups`), `TestListPlaygroupsPage_DoesNotPopulateMembers`
  (documents the one behavioral difference), `TestListPlaygroupsPage_PaginatesWithCursor`
  (walks 5 playgroups 2 at a time via `next_cursor`, same shape as
  `decks.TestListDecks_PaginatesWithCursor`, asserting no group is missed or
  repeated across pages), and `TestListPlaygroupsPage_InvalidCursor`.
- Verified the same way as the Stage 8 fix: `sqlc generate` (caught the
  missing `::uuid` cast noted above), `go build`/`go vet` clean,
  `golangci-lint run --timeout=5m` clean (only the same pre-existing
  repo-wide `gofmt`/CRLF noise), and `go test -race -cover -p 1 ./...`
  against a fresh throwaway `postgres:18-alpine` container passed for all
  packages, `internal/playgroups` coverage going from 47.6% to 51.8%.

### Stage 9 — Standalone Swiss-format Commander tournaments (2026-08-09)

The user asked directly for web-created tournaments (player count, 3-4
seats/table, Swiss format, 2/1/0/0 scoring, a join code usable in-app to
find your table) — pulling Stage 9 forward ahead of friends/groups, which
stay undesigned. Given the size (new domain model, a pairing algorithm,
backend + web across ~15 new files) this went through `EnterPlanMode`
first: two research agents (backend `games`/`decks`/playgroups schema and
patterns; web composable/page/i18n conventions) fed a written plan the user
approved before any code was touched. Mid-plan, one more question was put
to the user directly — whether "entering the match" via the code should
land the player in the *existing* live life-tracker (`games`/`game_players`)
or just show a read-only table assignment — because the honest answer
required checking two real constraints first (guest seats have no home in
`game_players.user_id`; ADR-0013's proxy-join only authorizes seating
someone if organizer and participant share a `playgroup_id`, which usually
isn't true for a stranger who joined by code). The user picked the
read-only lookup. Full design and every alternative considered is in
[ADR-0016](../decisions/0016-swiss-tournament-format.md) — this entry only
covers the how/verification.

- **Schema**: migration `00016_tournaments.sql`, five tables
  (`tournaments`, `tournament_participants`, `tournament_rounds`,
  `tournament_tables`, `tournament_table_seats`), fully self-contained (no
  FK to/from `games`/`game_players`). RLS enabled on all five (deny-all,
  same as every other table since the 2026-08-01 audit found
  `deck_resync_jobs` missing it — see the Stage 2 entry below) — added
  proactively this time instead of waiting for the Supabase advisor to
  flag it. `docs/database/schema.dbml` updated in the same pass, not left
  to drift (the Stage 2 entry below is exactly the failure mode this
  avoids).
- **Backend** (`backend/internal/tournaments`): scaffolded with real
  `sqlc generate` in Docker from the start this time (query.sql written,
  then generated for real, not hand-written-then-checked like the Stage 8
  pass) — `db.go`/`models.go`/`querier.go`/`query.sql.go` are genuine sqlc
  output. `pairing.go` holds the Swiss heuristic (table-size distribution,
  round-count staircase, greedy repeat-avoiding pairing, join-code
  generation) as pure, DB-free logic — see the ADR for exactly what it
  does and doesn't guarantee. `service.go` orchestrates registration,
  starting (transactional: lock roster + seat round 1), recording a
  table's result (transactional: validate the permutation, update seats,
  add points), and advancing a round (transactional: finish the round,
  either seat the next one or finish the tournament).
- **API**: 9 new endpoints under `/tournaments*`
  (`docs/api/openapi.yaml` updated with full schemas and paths). Verified
  with the same Dockerized Spectral lint used for the Stage 3 pagination
  work: 0 new errors, warnings went from 116 to 134 (18 = 9 new operations
  × the same missing-`operationId`/`tags` warnings every other endpoint in
  the spec already has — not a regression, the established baseline).
- **Web**: `types/api.ts` additions, `composables/useTournaments.ts`
  (mirrors `usePlaygroups.ts`'s shape exactly — one function per endpoint
  plus a matching `*Error(err)` helper per fallible call), two pages
  (`pages/tournaments/index.vue` list + create + join-by-code modals,
  `pages/tournaments/[id].vue` phase-driven detail: registration/
  in_progress/finished), a nav link, and `common.back` added as a new
  shared key (didn't exist before — every other back-link had its own
  page-scoped translation). i18n added programmatically: a one-off Node
  script parsed and rewrote `es.json`/`en.json`/`ca.json` (preserving
  their existing 2-space/CRLF formatting) rather than hand-editing ~150
  new keys across three files by hand — deleted after running.
- **Tests**: `pairing_internal_test.go` (package-internal, pure logic, no
  DB) locks in the two mathematically interesting pairing cases found by
  hand-tracing the greedy algorithm: 9 players in three 3-tables *can*
  reach a fully repeat-free round 2 (a 3×3 Latin-square shape) and the
  heuristic finds it; 8 players in two 4-tables *cannot* (each new
  4-table can fit at most one "safe" pick from each of the two original
  groups — 2 safe seats, short of 4) and the heuristic still produces a
  complete, valid pairing instead of erroring. `service_test.go` covers
  creation, self-service join (including the deck-ownership check
  reusing `internal/decks`), guest registration, authorization
  ("don't reveal" 404s for non-organizers), invalid roster counts (2, and
  5 — the one gap above the 3-player minimum), and a full 3-round,
  3-participant tournament lifecycle test that plays every round to
  completion and asserts the tournament reaches `finished` with the
  correct total points awarded.
- **Lint**: this pass produced real `golangci-lint` findings beyond the
  Stage 8 pass's few — `cyclop` (2 functions in `service.go` over the
  complexity limit, fixed by extracting helpers: `resolveTableResult`/
  `applyTableResult` out of `RecordTableResult`, `findParticipant`/
  `findCurrentTable` out of `LookupByCode`), `mnd` (named constants added
  for the round-count staircase and point values), `err113` (two
  `fmt.Errorf` calls building dynamic errors inline, fixed with static
  wrapped sentinels), several `govet` shadow warnings in both `service.go`
  and the tests (renamed or switched `:=` to `=`), and `goconst`/
  `gocritic`/`prealloc`/`staticcheck` nits in the tests. All fixed;
  `internal/tournaments` alone lints at 0 issues, full repo unchanged at
  the 68 pre-existing `gofmt`/CRLF findings.
- **Full verification**: `docker run ... sqlc/sqlc:1.27.0 generate`,
  `golang:1.25-alpine` build/vet, `golangci-lint:v2.12.2`, a throwaway
  `postgres:18-alpine` + migrations + `go test -race -cover -p 1 ./...`
  (all 17 packages green, `internal/tournaments` at 65.7% coverage), the
  Spectral OpenAPI lint, and `node:22-alpine` running the web app's real
  `npm run lint`/`npm run typecheck` (via an isolated Docker volume for
  `node_modules` so the container's Linux install never touched the
  Windows host's `web/node_modules`) — one real typecheck error caught
  (`SeatResultInput` imported from the wrong module) and fixed.

### Stage 1 — Playgroups: proxy-join and `GET /users/search`

See the 2026-07-28 audit-history entries above for the full narrative —
[ADR-0013](../decisions/0013-proxy-join-and-action-authorization.md)
covers the authorization design in detail. Worth calling out here: the
`GET /users/search?q=` endpoint matches `username` by `ILIKE` (partial,
case-insensitive) but `email` only by **exact** match, deliberately —
partial email matching would let one account enumerate other users'
addresses by prefix/substring. It never returns a third party's email in
the result shape (`UserSearchResult{id, username}`, no `email` field,
unlike the full `UserResponse`), excludes the requester themselves, and
caps results at 10 with a 2-character minimum query.

### Stage 1 — Auth: token strategy and Google Sign-In

Full design rationale in [ADR-0001](../decisions/0001-auth-jwt-refresh-token-strategy.md)
(why HS256 over RS256 in a monolith with no external token verifiers) and
[ADR-0002](../decisions/0002-google-sign-in.md) (why `go-oidc` over
`google.golang.org/api/idtoken` — same verification guarantees,
substantially lighter dependency footprint, avoids pulling in all of
`google.golang.org/api` + gRPC + OpenTelemetry to verify one token).
Account linking (`users.FindOrCreateGoogleUser`) looks up by `google_id`
first, then by email with **auto-link only if Google confirms
`email_verified`**, then falls back to creating a new passwordless account
(username derived from the email's local part, retried with a suffix on
collision).

### Stage 4/5 — Known, deliberate Android test gaps

Several `ViewModel`s have no dedicated unit test, all for the same
underlying reason: `SettingsViewModel`, `DashboardViewModel`,
`LoginViewModel`, `RegisterViewModel`, and `JoinGameViewModel` all inject
`SessionManager` directly — a concrete class with a `Context` constructor,
unfakeable in a pure-JVM test without Robolectric (which the project
doesn't use; `testing/Fakes.kt` are hand-written fakes against interfaces
only). Extracting a narrow interface (something like a
`SessionTokenProvider`) would solve it — it was tried and reverted during
the Settings-screen pass, documented there instead of left half-done — but
it's a real architectural change, not part of the domain-layer refactor
that did happen (see the 2026-08-01 domain-layer entry above, which
deliberately left auth out of scope for the same reason).
