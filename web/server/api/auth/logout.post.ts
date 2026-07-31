/**
 * Logs out: revokes the refresh token on the Go API (best-effort) and deletes
 * the three session cookies.
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
      // Best-effort: we clear the local session even if the API fails.
    }
  }

  clearSessionCookies(event)
  return { ok: true }
})
