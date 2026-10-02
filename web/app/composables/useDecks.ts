import type { Deck, DeckResyncJob, DeckTraitFilter, ManaColor, PaginatedResponse, SyncResponse, UpdateDeckBody } from '~/types/api'

export function useDecks() {
  const { apiFetch } = useApi()

  /** One page of the authenticated user's decks (default 20). Pass the
   * previous page's `next_cursor` to get the next one; omit it for the first
   * page. For the full list, use listAllDecks. `filter` narrows it server-side;
   * pass the same one with every cursor of a listing. */
  function listDecksPage(cursor?: string, filter?: DeckTraitFilter): Promise<PaginatedResponse<Deck>> {
    const query = { ...(filter ? deckTraitFilterQuery(filter) : {}), ...(cursor ? { cursor } : {}) }
    return apiFetch<PaginatedResponse<Deck>>('/decks', { query })
  }

  /**
   * Follows next_cursor until every page is fetched. Used where the real
   * total matters (dashboard's deck count stat, the per-deck stats page) --
   * a plain first page silently undercounts/omits decks past it.
   */
  async function listAllDecks(): Promise<Deck[]> {
    const all: Deck[] = []
    let cursor: string | undefined
    do {
      const page = await listDecksPage(cursor)
      all.push(...page.items)
      cursor = page.next_cursor ?? undefined
    } while (cursor)
    return all
  }

  /**
   * Creates a deck by hand. `imageUrl` is the art of the commander picked from
   * the Scryfall typeahead — omitted when the user typed a name instead of
   * choosing a suggestion, which leaves the deck on DeckArt's placeholder.
   * `bracket`/`colorIdentity` are optional; null (unknown) is left out.
   */
  function createDeck(input: {
    name: string
    commander: string
    imageUrl?: string | null
    bracket?: number | null
    colorIdentity?: ManaColor[] | null
  }) {
    return apiFetch<Deck>('/decks', {
      method: 'POST',
      body: {
        name: input.name.trim(),
        commander: input.commander.trim(),
        ...(input.imageUrl ? { image_url: input.imageUrl } : {}),
        ...(input.bracket != null ? { bracket: input.bracket } : {}),
        ...(input.colorIdentity ? { color_identity: input.colorIdentity } : {}),
      },
    })
  }

  /**
   * Sets or resets a deck's bracket and color identity. A value set here sticks
   * through Moxfield resyncs until it's reset (see UpdateDeckBody).
   */
  function updateDeck(id: string, body: UpdateDeckBody) {
    // Spread into a fresh literal: apiFetch wants a Record, which an interface isn't.
    return apiFetch<Deck>(`/decks/${id}`, { method: 'PATCH', body: { ...body } })
  }

  /** `input` accepts either the full Moxfield URL or just the public ID. */
  function importFromMoxfield(input: string) {
    return apiFetch<Deck>('/decks/import/moxfield', {
      method: 'POST',
      body: { url: input.trim() },
    })
  }

  /**
   * Re-syncs an already imported deck with its current version on Moxfield
   * (name, commander, image, and bracket/color identity unless set by hand). `moxfieldId` accepts either the public ID
   * or the full URL, same as import.
   */
  function syncFromMoxfield(moxfieldId: string) {
    return apiFetch<SyncResponse>('/sync/moxfield', {
      method: 'POST',
      body: { moxfield_id: moxfieldId },
    })
  }

  /** Triggers in the background a resync of ALL of the user's own decks that have a moxfield_id. */
  function resyncAllDecks() {
    return apiFetch<DeckResyncJob>('/decks/resync-all', { method: 'POST' })
  }

  function getResyncAllStatus(jobId: string) {
    return apiFetch<DeckResyncJob>(`/decks/resync-all/${jobId}`)
  }

  return {
    listDecksPage,
    listAllDecks,
    createDeck,
    updateDeck,
    importFromMoxfield,
    syncFromMoxfield,
    resyncAllDecks,
    getResyncAllStatus,
  }
}

/**
 * Translates POST /decks errors. The backend validates that name and
 * commander aren't blank (see internal/decks/service.go), which is the only
 * 400 a client that fills both can realistically hit.
 */
export function createDeckError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  if (apiErrorStatus(err) === 400) return t('errors.createDeck.missingFields')
  return apiErrorMessage(err, t('errors.createDeck.generic'))
}

/**
 * Translates Moxfield import errors into something actionable.
 * The backend responds 404 if there's no public deck with that ID, 400 if the
 * URL is invalid or the deck has no commander (not Commander format), and 409
 * if the user already imported this same Moxfield deck.
 */
export function moxfieldImportError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.moxfieldImport.notFound')
    case 409:
      return t('errors.moxfieldImport.alreadyImported')
    case 400:
      // The '' is a sentinel to test apiErrorMessage(...).includes('commander')
      // against the backend's raw English text — it's not a visible message, it's not translated.
      return apiErrorMessage(err, '').includes('commander')
        ? t('errors.moxfieldImport.notCommander')
        : t('errors.moxfieldImport.invalidUrl')
    default:
      return apiErrorMessage(err, t('errors.moxfieldImport.generic'))
  }
}

/** Translates POST /decks/resync-all errors. */
export function resyncAllDecksError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.resyncAllDecks.noneEligible')
    case 409:
      return t('errors.resyncAllDecks.inProgress')
    default:
      return apiErrorMessage(err, t('errors.resyncAllDecks.generic'))
  }
}

/** Translates PATCH /decks/{id} errors; 503 is a reset that needed Moxfield while it was down. */
export function updateDeckError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  if (apiErrorStatus(err) === 503) return t('errors.updateDeck.moxfieldUnavailable')
  return apiErrorMessage(err, t('errors.updateDeck.generic'))
}
