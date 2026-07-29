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

/** Traduce el status de una partida a una etiqueta legible en español. */
export function gameStatusLabel(status: Game['status']): string {
  switch (status) {
    case 'pending':
      return 'Pendiente'
    case 'active':
      return 'En curso'
    case 'finished':
      return 'Finalizada'
    default:
      return status
  }
}

/** 404 acá cubre tanto "el grupo no existe" como "no sos miembro" (ver games.ErrPlaygroupNotFound). */
export function listPlaygroupGamesError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 404:
      return 'El grupo no existe o no sos miembro.'
    default:
      return apiErrorMessage(err, 'No se pudo cargar el historial de partidas.')
  }
}
