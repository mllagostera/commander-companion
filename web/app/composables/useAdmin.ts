import type {
  AdminDailyActivityPoint,
  AdminOverviewStats,
  AdminUnfinishedGame,
  AdminUserDetail,
  AdminUserSummary,
  PaginatedResponse,
} from '~/types/api'

/**
 * Admin dashboard API client. Every call here hits an endpoint gated by
 * auth.RequireAdmin on the backend (see ADR-0018) — a non-admin gets a 403,
 * which callers surface with adminError() same as any other API error.
 */
export function useAdmin() {
  const { apiFetch } = useApi()

  /** One page of users, most recently created first. Pass the previous page's next_cursor for the next one. */
  function listUsers(cursor?: string, search?: string) {
    const query: Record<string, string> = {}
    if (cursor) query.cursor = cursor
    if (search) query.search = search
    return apiFetch<PaginatedResponse<AdminUserSummary>>('/admin/users', { query })
  }

  function getUser(id: string) {
    return apiFetch<AdminUserDetail>(`/admin/users/${id}`)
  }

  /** Activates/deactivates a user's account. The backend rejects deactivating your own account. */
  function updateUserStatus(id: string, isActive: boolean) {
    return apiFetch<AdminUserDetail>(`/admin/users/${id}/status`, {
      method: 'PATCH',
      body: { is_active: isActive },
    })
  }

  function getOverviewStats() {
    return apiFetch<AdminOverviewStats>('/admin/stats/overview')
  }

  /** Historical daily series for the activity chart, oldest day first. days defaults to 30 (clamped to [1, 90] server-side). */
  function getDailyActivity(days?: number) {
    return apiFetch<AdminDailyActivityPoint[]>('/admin/stats/activity', {
      query: days ? { days } : undefined,
    })
  }

  /** One page of games opened but never finished, oldest first. status narrows it to one of the two unfinished states. */
  function listUnfinishedGames(cursor?: string, status?: 'pending' | 'active') {
    const query: Record<string, string> = {}
    if (cursor) query.cursor = cursor
    if (status) query.status = status
    return apiFetch<PaginatedResponse<AdminUnfinishedGame>>('/admin/games', { query })
  }

  /** Deletes a pending/active game. The backend rejects a finished one with a 409. */
  function deleteUnfinishedGame(id: string) {
    return apiFetch<null>(`/admin/games/${id}`, { method: 'DELETE' })
  }

  return {
    listUsers,
    getUser,
    updateUserStatus,
    getOverviewStats,
    getDailyActivity,
    listUnfinishedGames,
    deleteUnfinishedGame,
  }
}

export function adminError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('admin.errors.cannotDeactivateSelf')
    case 403:
      return t('admin.errors.forbidden')
    case 404:
      return t('admin.errors.userNotFound')
    default:
      return apiErrorMessage(err, t('admin.errors.generic'))
  }
}

/** Same as adminError, with the messages that fit a game deletion instead of a user update. */
export function adminGameError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 404:
      return t('admin.errors.gameNotFound')
    case 409:
      return t('admin.errors.gameFinished')
    default:
      return adminError(err)
  }
}
