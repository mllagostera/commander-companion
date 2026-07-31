import type { AuthUser } from '../../utils/backend'

/**
 * Returns the current session's user, or `{ user: null }` if there isn't
 * one. Never fails with 401: it's the endpoint the hydration plugin queries
 * on every load, and a 401 there would just generate noise.
 *
 * If the access token expired, `backendFetch` transparently renews it
 * with the refresh token and updates the cookies.
 */
export default defineEventHandler(async (event) => {
  const hasTokens =
    !!getCookie(event, ACCESS_COOKIE) || !!getCookie(event, REFRESH_COOKIE)

  if (!hasTokens) {
    clearSessionCookies(event)
    return { user: null }
  }

  try {
    const user = await backendFetch<AuthUser>(event, '/auth/me')
    return { user }
  } catch {
    clearSessionCookies(event)
    return { user: null }
  }
})
