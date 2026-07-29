import type { Game, PaginatedResponse } from '~/types/api'

export function useGames() {
  const { apiFetch } = useApi()

  /**
   * Historial completo (sin paginar, ver ListGamesForPlaygroup en el backend) de
   * partidas de un grupo, con jugadores poblados. Requiere ser miembro del grupo.
   */
  function listPlaygroupGames(playgroupId: string) {
    return apiFetch<PaginatedResponse<Game>>('/games', {
      query: { playgroup_id: playgroupId },
    }).then((page) => page.items)
  }

  return { listPlaygroupGames }
}

/** Traduce el status de una partida a una etiqueta legible. */
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

/** 404 acá cubre tanto "el grupo no existe" como "no eres miembro" (ver games.ErrPlaygroupNotFound). */
export function listPlaygroupGamesError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.games.notFoundOrNotMember')
    default:
      return apiErrorMessage(err, t('errors.games.generic'))
  }
}
