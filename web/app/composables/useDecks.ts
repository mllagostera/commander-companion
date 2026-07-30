import type { Deck, DeckResyncJob, PaginatedResponse, SyncResponse } from '~/types/api'

export function useDecks() {
  const { apiFetch } = useApi()

  /** Devuelve la primera página entera (default 20 decks); no hay UI de paginación todavía. */
  async function listDecks(): Promise<Deck[]> {
    const page = await apiFetch<PaginatedResponse<Deck>>('/decks')
    return page.items
  }

  /** `input` acepta la URL completa de Moxfield o solo el ID público. */
  function importFromMoxfield(input: string) {
    return apiFetch<Deck>('/decks/import/moxfield', {
      method: 'POST',
      body: { url: input.trim() },
    })
  }

  /**
   * Re-sincroniza un deck ya importado con su versión actual en Moxfield
   * (nombre, comandante e imagen). `moxfieldId` acepta tanto el ID público
   * como la URL completa, igual que el import.
   */
  function syncFromMoxfield(moxfieldId: string) {
    return apiFetch<SyncResponse>('/sync/moxfield', {
      method: 'POST',
      body: { moxfield_id: moxfieldId },
    })
  }

  /** Dispara en background el resync de TODOS los decks propios con moxfield_id. */
  function resyncAllDecks() {
    return apiFetch<DeckResyncJob>('/decks/resync-all', { method: 'POST' })
  }

  function getResyncAllStatus(jobId: string) {
    return apiFetch<DeckResyncJob>(`/decks/resync-all/${jobId}`)
  }

  return { listDecks, importFromMoxfield, syncFromMoxfield, resyncAllDecks, getResyncAllStatus }
}

/**
 * Traduce los errores del import de Moxfield a algo accionable.
 * El backend responde 404 si no existe un deck público con ese ID y 400 si la
 * URL no es válida o el deck no tiene comandante (no es formato Commander).
 */
export function moxfieldImportError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.moxfieldImport.notFound')
    case 400:
      // El '' es un sentinel para probar apiErrorMessage(...).includes('commander')
      // contra el texto crudo en inglés del backend — no es un mensaje visible, no se traduce.
      return apiErrorMessage(err, '').includes('commander')
        ? t('errors.moxfieldImport.notCommander')
        : t('errors.moxfieldImport.invalidUrl')
    default:
      return apiErrorMessage(err, t('errors.moxfieldImport.generic'))
  }
}

/** Traduce los errores de POST /decks/resync-all. */
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
