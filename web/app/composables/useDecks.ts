import type { Deck } from '~/types/api'

export function useDecks() {
  const { apiFetch } = useApi()

  function listDecks() {
    return apiFetch<Deck[]>('/decks')
  }

  /** `input` acepta la URL completa de Moxfield o solo el ID público. */
  function importFromMoxfield(input: string) {
    return apiFetch<Deck>('/decks/import/moxfield', {
      method: 'POST',
      body: { url: input.trim() },
    })
  }

  return { listDecks, importFromMoxfield }
}

/**
 * Traduce los errores del import de Moxfield a algo accionable.
 * El backend responde 404 si no existe un deck público con ese ID y 400 si la
 * URL no es válida o el deck no tiene comandante (no es formato Commander).
 */
export function moxfieldImportError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 404:
      return 'No existe un deck público de Moxfield con ese ID. Revisá la URL y que el deck no sea privado.'
    case 400:
      return apiErrorMessage(err, '').includes('commander')
        ? 'Ese deck de Moxfield no tiene comandante: no parece ser un deck de formato Commander.'
        : 'La URL o el ID de Moxfield no son válidos. Pegá algo como https://moxfield.com/decks/abc123 o solo abc123.'
    default:
      return apiErrorMessage(err, 'No se pudo importar el deck desde Moxfield.')
  }
}
