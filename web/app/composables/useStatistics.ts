import type {
  DeckStats,
  FinishedGame,
  OpponentStats,
  PaginatedResponse,
  PlaygroupGameCount,
  PlaygroupStats,
  UserStats,
} from '~/types/api'

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

  /**
   * Every playgroup the user belongs to, with its games_played count, in one
   * request -- replaces calling playgroupStats once per group just to find
   * the one played the most.
   */
  function playgroupGameCounts() {
    return apiFetch<PlaygroupGameCount[]>('/statistics/playgroups')
  }

  /** Head-to-head record against every opponent the user has shared a finished game with. */
  function opponentStats() {
    return apiFetch<OpponentStats[]>('/statistics/opponents')
  }

  /** One page of the finished-games history, most recent first. Pass the previous page's next_cursor for the next one. */
  function listFinishedGames(cursor?: string) {
    return apiFetch<PaginatedResponse<FinishedGame>>('/statistics/games', { query: cursor ? { cursor } : undefined })
  }

  return { userStats, deckStats, allDeckStats, playgroupStats, playgroupGameCounts, opponentStats, listFinishedGames }
}

/** Formatted win percentage, tolerant of 0 games. */
export function winRate(gamesPlayed: number, gamesWon: number): string {
  if (!gamesPlayed) return '—'
  return `${Math.round((gamesWon / gamesPlayed) * 100)}%`
}
