import type { DeckStats, PlaygroupStats, UserStats } from '~/types/api'

export function useStatistics() {
  const { apiFetch } = useApi()

  /** Siempre disponible para el usuario autenticado (ceros si nunca jugó). */
  function userStats() {
    return apiFetch<UserStats>('/statistics/user')
  }

  function deckStats(deckId: string) {
    return apiFetch<DeckStats>(`/statistics/deck/${deckId}`)
  }

  /** Consumido desde pages/playgroups/[id].vue. Best-effort: 404 si el grupo nunca jugó. */
  function playgroupStats(playgroupId: string) {
    return apiFetch<PlaygroupStats>(`/statistics/playgroup/${playgroupId}`)
  }

  return { userStats, deckStats, playgroupStats }
}

/** Porcentaje de victorias formateado, tolerante a 0 partidas. */
export function winRate(gamesPlayed: number, gamesWon: number): string {
  if (!gamesPlayed) return '—'
  return `${Math.round((gamesWon / gamesPlayed) * 100)}%`
}
