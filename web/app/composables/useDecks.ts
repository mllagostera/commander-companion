import type { Deck, DeckResyncJob, PaginatedResponse, SyncResponse } from '~/types/api'

export function useDecks() {
  const { apiFetch } = useApi()

  /**
   * Returns just the first page (default 20 decks). Used where the full list
   * isn't needed (dashboard/statistics deck pickers) -- for a page that needs
   * every deck (browsing, search), use listDecksPage and follow next_cursor.
   */
  async function listDecks(): Promise<Deck[]> {
    const page = await apiFetch<PaginatedResponse<Deck>>('/decks')
    return page.items
  }

  /** One page of the authenticated user's decks. Pass the previous page's
   * `next_cursor` to get the next one; omit it for the first page. */
  function listDecksPage(cursor?: string): Promise<PaginatedResponse<Deck>> {
    return apiFetch<PaginatedResponse<Deck>>('/decks', { query: cursor ? { cursor } : undefined })
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
   * (name, commander and image). `moxfieldId` accepts either the public ID
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

  return { listDecks, listDecksPage, importFromMoxfield, syncFromMoxfield, resyncAllDecks, getResyncAllStatus }
}

/**
 * Translates Moxfield import errors into something actionable.
 * The backend responds 404 if there's no public deck with that ID, and 400 if the
 * URL is invalid or the deck has no commander (not Commander format).
 */
export function moxfieldImportError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.moxfieldImport.notFound')
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
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.resyncAllDecks.noneEligible')
    case 409:
      return t('errors.resyncAllDecks.inProgress')
    default:
      return apiErrorMessage(err, t('errors.resyncAllDecks.generic'))
  }
}
