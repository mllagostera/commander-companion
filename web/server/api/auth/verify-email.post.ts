/**
 * Confirms the account from the token that arrived by email. A thin proxy to
 * the Go API's `POST /auth/verify-email`: the token travels in the body (not in a query
 * string) because the email links to this same client's `/verify-email?token=...`
 * page, which is what makes this POST (see internal/users/handler.go: VerifyEmail).
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{ token?: string }>(event)

  if (!body?.token) {
    const message = 'Falta el token de verificación.'
    throw createError({
      statusCode: 400,
      statusMessage: message,
      message,
      data: { code: 400, message },
    })
  }

  try {
    await $fetch('/auth/verify-email', {
      baseURL: backendBase(event),
      method: 'POST',
      body: { token: body.token },
    })
  } catch (err) {
    throw toBackendError(err)
  }

  return { verified: true }
})
