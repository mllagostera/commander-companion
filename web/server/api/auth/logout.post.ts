/**
 * Cierra la sesión: revoca el refresh token en la API Go (best-effort) y borra
 * las tres cookies de sesión.
 */
export default defineEventHandler(async (event) => {
  const refreshToken = getCookie(event, REFRESH_COOKIE)

  if (refreshToken) {
    try {
      await $fetch('/auth/logout', {
        baseURL: backendBase(event),
        method: 'POST',
        body: { refresh_token: refreshToken },
      })
    } catch {
      // Best-effort: limpiamos la sesión local aunque la API falle.
    }
  }

  clearSessionCookies(event)
  return { ok: true }
})
