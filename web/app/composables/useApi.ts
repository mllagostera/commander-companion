import type { NitroFetchArgs } from './useNitroFetch'

/**
 * Go API client for the screens.
 *
 * Never calls the API directly: it goes through Nitro's `/api/backend/**`
 * proxy, which is the one that knows the access token (httpOnly cookie) and the
 * one that renews it against `POST /auth/refresh` if it expired.
 *
 * A 401 here means it couldn't even be refreshed (revoked or expired refresh
 * token): we close the local session and send the user to login.
 */
export function useApi() {
  const nitroFetch = useNitroFetch()
  const { resetSession } = useAuth()

  async function apiFetch<T>(
    path: string,
    options: NitroFetchArgs = {},
  ): Promise<T> {
    try {
      return await nitroFetch<T>(`/api/backend${path}`, options)
    } catch (err) {
      if (apiErrorStatus(err) === 401) {
        resetSession()
        if (import.meta.client) {
          await navigateTo('/login')
        }
      }
      throw err
    }
  }

  return { apiFetch }
}

/** HTTP status of an ofetch/h3 error, regardless of which layer it comes from. */
export function apiErrorStatus(err: unknown): number | undefined {
  const e = err as { status?: number; statusCode?: number }
  return e?.status ?? e?.statusCode
}

/** Readable message from an API error, with a fallback. */
export function apiErrorMessage(err: unknown, fallback: string): string {
  const e = err as {
    data?: { message?: string }
    statusMessage?: string
    message?: string
  }
  return e?.data?.message || e?.statusMessage || fallback
}
