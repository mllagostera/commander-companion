import type { AuthUser } from '../../utils/backend'

/**
 * Devuelve el usuario de la sesión actual, o `{ user: null }` si no hay
 * ninguna. Nunca falla con 401: es el endpoint que consulta el plugin de
 * hidratación en cada carga, y un 401 ahí solo generaría ruido.
 *
 * Si el access token expiró, `backendFetch` lo renueva de forma transparente
 * con el refresh token y actualiza las cookies.
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
