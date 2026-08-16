/**
 * Commander typeahead for the "new deck" form, backed by Scryfall.
 *
 * This runs in Nitro rather than in the browser for three separate reasons,
 * any one of which would be enough:
 *
 *  - The app's own CSP sets `connect-src 'self'` (see
 *    server/utils/security-headers.ts). A `fetch('https://api.scryfall.com/...')`
 *    from the page would simply be blocked, and widening connect-src to a
 *    third party to power a form field is a bad trade.
 *  - Scryfall's API guidelines ask clients to identify themselves with a
 *    `User-Agent`. A browser can't set that header; Node can.
 *  - `/cards/search` answers with **full** card objects — every printing,
 *    legality, price and ruling. For a dropdown we need two fields, so the
 *    payload is trimmed here instead of shipping ~100× the bytes to the
 *    client on every keystroke.
 *
 * Nothing here is persisted: the picked name and art URL are sent by the
 * client to the API's own `POST /decks` like any other manually typed deck.
 */

import type { CommanderSuggestion } from '#shared/types/scryfall'

/** Shape of the bits of a Scryfall card this endpoint reads. */
interface ScryfallCard {
  name?: string
  type_line?: string
  image_uris?: { art_crop?: string }
  /** Populated instead of `image_uris` on double-faced cards. */
  card_faces?: { image_uris?: { art_crop?: string } }[]
}

interface ScryfallSearchResponse {
  data?: ScryfallCard[]
}

const SCRYFALL_SEARCH = 'https://api.scryfall.com/cards/search'
const MAX_RESULTS = 10
/** Below this, a prefix matches most of the format and the ranking is noise. */
const MIN_QUERY_LENGTH = 2
const CACHE_TTL_MS = 10 * 60 * 1000
const CACHE_MAX_ENTRIES = 200

const cache = new Map<string, { at: number, results: CommanderSuggestion[] }>()

function cacheGet(key: string): CommanderSuggestion[] | null {
  const hit = cache.get(key)
  if (!hit) return null
  if (Date.now() - hit.at > CACHE_TTL_MS) {
    cache.delete(key)
    return null
  }
  return hit.results
}

function cacheSet(key: string, results: CommanderSuggestion[]) {
  // Map iterates in insertion order, so the first key is the oldest. Plain
  // FIFO rather than LRU: entries expire on a timer anyway, and the cap only
  // exists so a long uptime can't grow this without bound.
  if (cache.size >= CACHE_MAX_ENTRIES) {
    const oldest = cache.keys().next().value
    if (oldest !== undefined) cache.delete(oldest)
  }
  cache.set(key, { at: Date.now(), results })
}

/**
 * Wraps the user's text as a quoted `name:` term.
 *
 * Quoting is what makes multi-word input work (`name:"jodah, the"`), and
 * stripping `"` and `\` is what keeps the user from breaking out of the quotes
 * and appending their own Scryfall operators to the query we build.
 */
function toScryfallQuery(raw: string): string {
  const cleaned = raw.replace(/["\\]/g, ' ').trim()
  return `is:commander name:"${cleaned}"`
}

function artOf(card: ScryfallCard): string | null {
  return card.image_uris?.art_crop ?? card.card_faces?.[0]?.image_uris?.art_crop ?? null
}

export default defineEventHandler(async (event): Promise<CommanderSuggestion[]> => {
  const { q } = getQuery<{ q?: string }>(event)
  const query = (q ?? '').trim()

  if (query.length < MIN_QUERY_LENGTH) return []

  const key = query.toLowerCase()
  const cached = cacheGet(key)
  if (cached) return cached

  let response: ScryfallSearchResponse
  try {
    response = await $fetch<ScryfallSearchResponse>(SCRYFALL_SEARCH, {
      query: {
        q: toScryfallQuery(query),
        // EDHREC rank puts the commanders people actually build first, which
        // is what makes a 10-item dropdown useful instead of alphabetical.
        order: 'edhrec',
        // Otherwise every reprint of the same card is its own result.
        unique: 'cards',
      },
      headers: {
        'User-Agent': 'CommanderCompanion/1.0',
        'Accept': 'application/json',
      },
    })
  } catch (err) {
    // Scryfall answers 404 (not an empty 200) when a search matches nothing,
    // which for a typeahead is the normal state of a half-typed name — not an
    // error worth surfacing.
    if ((err as { status?: number, statusCode?: number })?.status === 404
      || (err as { statusCode?: number })?.statusCode === 404) {
      cacheSet(key, [])
      return []
    }
    throw createError({
      statusCode: 502,
      statusMessage: 'Scryfall lookup failed',
      message: 'Scryfall lookup failed',
      data: { code: 502, message: 'Scryfall lookup failed' },
    })
  }

  const results: CommanderSuggestion[] = (response.data ?? [])
    .slice(0, MAX_RESULTS)
    .filter((card): card is ScryfallCard & { name: string } => typeof card.name === 'string' && card.name !== '')
    .map((card) => ({
      name: card.name,
      type_line: card.type_line ?? '',
      image_url: artOf(card),
    }))

  cacheSet(key, results)
  return results
})
