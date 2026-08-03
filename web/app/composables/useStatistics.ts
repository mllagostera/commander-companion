import type { DeckStats, PlaygroupStats, UserStats } from '~/types/api'

export function useStatistics() {
  const { apiFetch } = useApi()

  /** Always available for the authenticated user (zeros if they never played). */
  function userStats() {
    return apiFetch<UserStats>('/statistics/user')
  }

  function deckStats(deckId: string) {
    return apiFetch<DeckStats>(`/statistics/deck/${deckId}`)
  }

  /**
   * Statistics for every deck the user owns, in a single request -- use this
   * instead of calling deckStats once per deck (that's what the dashboard and
   * the statistics page used to do, one HTTP round-trip per deck).
   */
  function allDeckStats() {
    return apiFetch<DeckStats[]>('/statistics/decks')
  }

  /** Consumed from pages/playgroups/[id].vue. Best-effort: 404 if the group never played. */
  function playgroupStats(playgroupId: string) {
    return apiFetch<PlaygroupStats>(`/statistics/playgroup/${playgroupId}`)
  }

  return { userStats, deckStats, allDeckStats, playgroupStats }
}

/** Formatted win percentage, tolerant of 0 games. */
export function winRate(gamesPlayed: number, gamesWon: number): string {
  if (!gamesPlayed) return '—'
  return `${Math.round((gamesWon / gamesPlayed) * 100)}%`
}
