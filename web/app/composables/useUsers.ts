import type { UserSearchResult } from '~/types/api'

export function useUsers() {
  const { apiFetch } = useApi()

  /**
   * Busca usuarios por username (parcial) o email (exacto) — para invitar gente a un
   * playgroup sin conocer su UUID. Nunca incluye al propio usuario autenticado ni el
   * email de nadie en el resultado (ver GET /users/search en docs/api/openapi.yaml).
   */
  function searchUsers(query: string) {
    return apiFetch<UserSearchResult[]>('/users/search', { query: { q: query } })
  }

  return { searchUsers }
}

/** Ver ErrSearchQueryTooShort (400) en internal/users/service.go. */
export function searchUsersError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.users.tooShort')
    default:
      return apiErrorMessage(err, t('errors.users.generic'))
  }
}
