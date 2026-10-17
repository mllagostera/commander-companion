# ADR-0029: Take imported deck art from Scryfall's CDN

**Status:** Accepted (2026-10-17)

## Context

Since Stage 8 an imported deck stores `decks.image_url` as an art crop on
Moxfield's asset CDN (`assets.moxfield.net/cards/card-{id}-art_crop.jpg`),
built from the `main` card of Moxfield's deck response. Every client hotlinks
that URL straight from the browser or the phone.

Two problems with that source:

- Moxfield's CDN is not a public image service. Nothing documents that
  hotlinking it is allowed, and on 2026-10-17 the web `/decks` grid showed the
  first page of art fine and empty tiles for the pages loaded on scroll, with
  identical URL shapes on both sides (checked against production data) —
  consistent with the CDN throttling a burst of requests, though not confirmed
  from this environment, whose network policy blocks the host.
- Two-faced cards need a separate `card-face-` prefix and the face's id; the
  plain prefix with that id silently serves an unrelated card.

Decks created by hand already store Scryfall art (`cards.scryfall.io`, picked
from the commander typeahead), and both the web CSP and Android's Coil loader
already serve that host.

## Decision

`moxfield.mainImageURL` builds the URL from the main card's `scryfall_id`:
`https://cards.scryfall.io/art_crop/front/{id[0]}/{id[1]}/{id}.jpg`. Scryfall
explicitly allows hotlinking its images, and `front` resolves two-faced cards
without the face-id handling. The id must be a canonical UUID before it reaches
the URL path.

When Moxfield reports no valid Scryfall id, the previous Moxfield CDN URL is kept
as the fallback, so a change in Moxfield's payload degrades to today's behaviour
instead of losing the art.

Alternatives considered:

- **Proxy and cache the images through the Nitro BFF.** Fixes hotlinking for the
  web only (Android loads the URL itself), and makes our server pay the bandwidth.
- **Look the art up on Scryfall's API by commander name.** An extra upstream call
  per import, ambiguous for partner pairs and for printings; the id Moxfield
  already returns is exact.
- **A one-off migration rewriting stored URLs.** Impossible: the Moxfield short id
  in the stored URL can't be mapped to a Scryfall id without asking Moxfield again.

## Consequences

- New imports, `/sync/moxfield` and `/decks/resync-all` store Scryfall art. Sync
  already treats a changed `image_url` as a change, so existing decks move over
  on their next update; until then they keep the Moxfield URL.
- The Scryfall URL omits the `?{timestamp}` cache-buster Scryfall's API adds.
  If Scryfall replaces a card's scan, clients may show the cached old one; the
  art crop of a printing practically never changes.
- The field name `scryfall_id` comes from Moxfield's undocumented API and was not
  confirmable live from the environment where this was written. If it is wrong,
  the fallback keeps today's behaviour; check a freshly imported deck's
  `image_url` after deploying.
