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

  /** Consumed from pages/playgroups/[id].vue. Best-effort: 404 if the group never played. */
  function playgroupStats(playgroupId: string) {
    return apiFetch<PlaygroupStats>(`/statistics/playgroup/${playgroupId}`)
  }

  return { userStats, deckStats, playgroupStats }
}

/** Formatted win percentage, tolerant of 0 games. */
export function winRate(gamesPlayed: number, gamesWon: number): string {
  if (!gamesPlayed) return '—'
  return `${Math.round((gamesWon / gamesPlayed) * 100)}%`
}
