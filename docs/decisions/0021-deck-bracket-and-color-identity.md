# ADR-0021: Deck bracket and color identity, overridable and snapshotted per seat

**Status:** Accepted (2026-10-02)

## Context

The repo owner wants to filter decks, statistics, game history and the pregame
deck picker by a deck's Commander bracket (Wizards' 1-5 power-level scale) and
by its color identity. Until now a deck stored only its name, commander,
`moxfield_id` and art.

Moxfield's deck endpoint (`GET api2.moxfield.com/v3/decks/all/{id}`, the one the
import already calls) returns both values: `bracket` (the one the author
declared), `autoBracket` (the one Moxfield computes; the two match unless the
author picked one by hand), `ignoreBrackets`, and `colorIdentity` as a list of
WUBRG letters. Manually created decks have no source for either.

Two questions needed an answer before building:

1. Who owns the value? The owner asked for both to be editable on **every**
   deck, including Moxfield ones, and for the "update" buttons (per-deck
   `POST /sync/moxfield` and `POST /decks/resync-all`) to keep pulling them in.
2. A deck's bracket drifts as it is tuned. Should "games played at bracket 2"
   mean the deck's bracket *today* or *when the game was played*? The owner
   chose the latter.

## Decision

**Storage.** `decks.bracket smallint` (1-5) and `decks.color_identity text[]`
(subset of `W,U,B,R,G`, stored in WUBRG order). NULL means unknown in both, and
`'{}'` means colorless, so "we don't know" is never confused with "colorless".
A text array won over a `"WUBR"` string because Postgres containment
(`@>`, `<@`) maps directly onto the three filter modes (`exact`, `includes`,
`within`) without string tricks.

**Ownership.** Two flags, `bracket_overridden` and `color_identity_overridden`.
Import and resync write Moxfield's values only while the matching flag is false
(the `CASE WHEN ..._overridden` in `UpdateDeckFromMoxfield`). `PATCH /decks/{id}`
sets a value (null clears it to unknown) and raises the flag;
`reset_bracket` / `reset_color_identity` lower it and fetch Moxfield's value in
the same request. Moxfield is fetched before anything is written, so an outage
changes nothing. The bracket taken from Moxfield is `bracket`, falling back to
`autoBracket`, and none when `ignoreBrackets` is set.

Rejected: a single "manual" flag for the whole deck, which would stop the
color identity from syncing just because the bracket was tuned; and a separate
`*_source` enum, which is the same information spelled longer.

**Per-seat snapshot.** `game_players.deck_bracket` / `deck_color_identity` are
copied from the deck when the seat is added (`games.AddGamePlayer`).
`GET /statistics/games` filters, and `GET /statistics/breakdown` groups, by the
caller's own seat snapshot, not by the deck's current value.

Seats added before a deck's value was known (every game before this migration,
and every game with a deck nobody has resynced yet) would otherwise stay
unknown forever. So whenever a deck's value becomes known (resync or PATCH),
`BackfillSeatDeckTraits` fills it into that deck's seats **that still have
none**. A seat that already has a value is never rewritten: it is what was
played.

Rejected: filtering by the deck's current value (simpler, but re-files old
games every time a deck is retuned, which is exactly what the owner didn't
want); and backfilling in the migration (nothing to backfill from: no deck had
values yet).

## Consequences

- One "update all" (`POST /decks/resync-all`) after deploying fills both the
  decks and their past seats, so existing statistics become filterable without a
  data migration.
- The backfill is a best guess for games played before the value was known: a
  deck retuned from bracket 2 to 3 before its first resync has its old games
  filed under 3. Accepted; there's no source for the older value.
- Decks of other users only get values when *their* owner imports or resyncs;
  an opponent's seat can stay unknown. Filters are always on the caller's own
  seat, so this only affects what an opponent's row shows.
- Brackets the author declared differ from Moxfield's computation only when the
  author chose so; we trust the author. If Moxfield changes the field names, the
  import stores unknown rather than failing, since both fields are optional.
- Clients (web, Android) need a follow-up to show, edit and filter by these
  values; this ADR covers the API and data model only.
