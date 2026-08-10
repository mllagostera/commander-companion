# Screenshot gallery — web client

Real screenshots of the web client (`web/`), captured against a local
instance running the actual backend + Postgres (Docker Compose) and a real
data flow: a freshly registered account, a playgroup with two members, a
3-participant Swiss tournament played from registration to its final
standings, and a local pass-and-play game. Nothing here is a mockup — every
screen is exactly what the app renders today.

**How these were captured**: a throwaway Playwright script drove a real
Chromium browser through each flow end-to-end and saved a screenshot at each
step (locale `es-ES`, viewport 1440×900, dark theme — the app's default).
The script isn't part of the repo; it exists only to produce this gallery
and gets re-run by hand whenever the UI changes enough to make these stale.
See [`wireframes.md`](wireframes.md) for the Android screens (no emulator
was available to capture those the same way — see `docs/roadmap/TASKS.md`)
and [`use-cases.md`](use-cases.md) for the step-by-step flows behind
each screen.

**Freshness note**: these are a point-in-time capture (2026-08-09). The web
client has no visual regression testing, so treat this gallery as
documentation, not as a guarantee the pixels still match — re-generate it
after any significant UI change.

---

## Authentication

### Login

![Login screen](screenshots/login.png)

Email/password sign-in plus "Continue with Google" (Credential Manager on
Android, Google Identity Services here). Shows the account-not-verified
recovery path (resend verification) when relevant.

### Register

![Register screen](screenshots/register.png)

Username/email/password with live username-availability checking
(debounced, shown inline under the field) and a password-confirmation
field.

### Check your email

![Post-registration screen](screenshots/register-check-email.png)

Shown instead of redirecting after a successful registration — the account
needs email verification before it can log in (in this local dev setup
verification is disabled by default, so the account is actually usable
immediately; production has it enabled).

---

## Dashboard

### Empty state

![Dashboard, no activity yet](screenshots/dashboard-empty.png)

First screen after login: stat cards (games/wins/win rate/decks), a
win-loss ring, best deck, current streak, groups, decks, and recent games —
all showing their empty states for a brand-new account.

### With activity

![Dashboard after joining a group](screenshots/dashboard-with-activity.png)

Same screen after creating a playgroup and adding a member — the "Tus
grupos" section now lists it.

### Account menu

![Top-right account dropdown: theme, language, settings, logout](screenshots/nav-user-menu.png)

Opened from the avatar in the top bar: dark-mode toggle, language switch
(es/en/ca — the same three locales as Android), a link to Settings, and
logout.

---

## Decks

### Empty state

![Decks page, no decks imported yet](screenshots/decks-empty.png)

Search, sort (played/won/win rate/name), and the "+ Agregar deck" action —
the only way to add a deck is a real Moxfield import (there's no manual
deck-entry form).

### Import modal

![Moxfield import modal](screenshots/decks-import-modal.png)

Takes a Moxfield deck URL or ID; on success it shows the imported deck's
name, commander, and art inline before closing.

---

## Playgroups

### List, empty

![Playgroups list, none yet](screenshots/playgroups-empty.png)

### Create modal

![Create-playgroup modal](screenshots/playgroups-create-modal.png)

### Detail

![Playgroup detail: ranking, members, history](screenshots/playgroup-detail.png)

Win-rate ranking table, a plain member list, and game history — all in
their empty states for a freshly created group. Renaming is available
inline via the "Renombrar" link next to the title.

### Add a member

![Searching for a user to add to the playgroup](screenshots/playgroup-add-member.png)

Adding a member is a username/email search against **existing registered
users** — there's no invite-by-email flow; the person has to already have
an account.

### With a member added

![Playgroup detail after adding a second member](screenshots/playgroup-with-member.png)

---

## Tournaments

Standalone Swiss-format events (pods of 3-4, not tied to a playgroup) —
see [ADR-0016](../decisions/0016-swiss-tournament-format.md) for the format
and scoring rules. This gallery walks one full tournament from creation to
its final standings.

### List, empty

![Tournaments list, none yet](screenshots/tournaments-empty.png)

### Create modal

![Create-tournament modal](screenshots/tournaments-create-modal.png)

### Registration phase

![Tournament detail during registration: join code, empty participant list](screenshots/tournament-registration.png)

The join code participants use to find the tournament (from the web or,
once built, Android). Organizer-only controls to add guest participants and
start the tournament once enough are registered.

### Participants added

![Three guest participants added, each with a commander](screenshots/tournament-participants.png)

Guests need only a name and commander (no account) — this run used 3 guests,
a valid pod size (3 players, one table) with no self-registered app user
needed to demonstrate the flow.

### Round 1, tables generated

![Round 1: one table, three seats, unrecorded finish positions](screenshots/tournament-round-tables.png)

Starting the tournament locks the round count (3 rounds for this player
count) and generates round 1's pairings. Each seat gets a `<select>` for
its finish position once the table is played out.

### Round 1, result recorded

![Round 1 result recorded, standings updated, "next round" available](screenshots/tournament-round-recorded.png)

Recording a table's result awards points (1st → 2, 2nd → 1, 3rd/4th → 0)
and immediately updates the standings below; "Siguiente ronda" becomes
available once every table in the round has a result.

### Finished, final standings

![Tournament finished after 3 rounds, winner highlighted](screenshots/tournament-finished-standings.png)

After the last round's results are recorded, the tournament status flips to
finished and the top standing gets a "Ganador" tag.

---

## Statistics

### Global

![Global statistics: win rate, best deck, damage totals](screenshots/statistics-global.png)

Aggregate stats, head-to-head ("cara a cara") against other players, and
the playgroup played most often — shown here in their empty states, since
this account's only recorded results came from the tournament above
(tournament results feed tournament standings, not personal game
statistics — those come from actual tracked games).

### Finished games tab

![Per-deck and finished-games tabs](screenshots/statistics-finished-games.png)

---

## Local life tracker (`/play`)

Explicitly a **local-only, unsaved** pass-and-play tracker — it doesn't
require login or record anything server-side, unlike the tournament/game
flows above. See [`use-cases.md`](use-cases.md) for how this compares
to Android's tracker (which does mirror the authenticated player's seat to
the backend, best-effort).

### Setup

![Player count picker and name fields](screenshots/play-setup.png)

### Setup, filled in

![Setup with 4 players named](screenshots/play-setup-filled.png)

### Pre-start

![Tracker screen before the "who goes first" randomizer runs](screenshots/play-tracker-prestart.png)

### Started

![Four-player quadrant layout, each at 40 life, "X empieza" banner](screenshots/play-tracker-started.png)

Each player's tile is rotated to face them around a shared table/device —
the top two quadrants are shown upside-down on purpose. Life adjusts via
tapping either half of a tile; a pass-turn button and turn counter sit at
the center.

### Commander damage overlay

![Expanded commander-damage and poison tracking for one seat](screenshots/play-tracker-commander-damage.png)

Tapping a quadrant's mini damage indicator expands per-opponent commander
damage counters and a poison counter, without leaving the main tracker.

### Summary

![Post-game summary: life/damage/poison/status per player](screenshots/play-summary.png)

Shown after finishing manually or via sudden death — winner (or draw)
banner plus a full per-player breakdown.

---

## Settings

![Username, Moxfield username, password change, logout](screenshots/settings.png)

Username, background Moxfield-username-based deck import, password change
(hidden if the account only has Google sign-in), and logout.
