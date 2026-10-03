/**
 * Sets a new password from the token that arrived by email. A thin proxy to the Go API's
 * `POST /auth/reset-password`: the token travels in the body (not in a query string),
 * same as verify-email.post.ts. It doesn't start a session — the backend revokes every
 * session of the account on reset, so the page sends the user to log in again.
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{ token?: string; new_password?: string }>(event)

  if (!body?.token || !body?.new_password) {
    const message = 'Missing token or new password.'
    throw createError({
      statusCode: 400,
      statusMessage: message,
      message,
      data: { code: 400, message },
    })
  }

  try {
    await $fetch('/auth/reset-password', {
      baseURL: backendBase(event),
      method: 'POST',
      body: { token: body.token, new_password: body.new_password },
    })
  } catch (err) {
    throw toBackendError(err)
  }

  // Any session this browser held is dead in the backend now; drop its cookies too.
  clearSessionCookies(event)
  return { reset: true }
})
