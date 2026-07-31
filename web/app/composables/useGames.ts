import type { Game, PaginatedResponse } from '~/types/api'

export function useGames() {
  const { apiFetch } = useApi()

  /**
   * Full history (unpaginated, see ListGamesForPlaygroup in the backend) of
   * a group's games, with players populated. Requires being a member of the group.
   */
  function listPlaygroupGames(playgroupId: string) {
    return apiFetch<PaginatedResponse<Game>>('/games', {
      query: { playgroup_id: playgroupId },
    }).then((page) => page.items)
  }

  return { listPlaygroupGames }
}

/** Translates a game's status into a readable label. */
export function gameStatusLabel(status: Game['status']): string {
  const { t } = useI18n()
  switch (status) {
    case 'pending':
      return t('gameStatus.pending')
    case 'active':
      return t('gameStatus.active')
    case 'finished':
      return t('gameStatus.finished')
    default:
      return status
  }
}

/** 404 here covers both "the group doesn't exist" and "you're not a member" (see games.ErrPlaygroupNotFound). */
export function listPlaygroupGamesError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.games.notFoundOrNotMember')
    default:
      return apiErrorMessage(err, t('errors.games.generic'))
  }
}
