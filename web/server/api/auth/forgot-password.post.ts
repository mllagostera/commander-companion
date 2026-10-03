/**
 * Asks for a password reset email. A thin proxy to the Go API's
 * `POST /auth/forgot-password`, which never reveals whether the email exists (see
 * internal/users/service.go: RequestPasswordReset) — this endpoint responds success in
 * every case.
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{ email?: string; locale?: string }>(event)

  if (!body?.email) {
    const message = 'Missing email.'
    throw createError({
      statusCode: 400,
      statusMessage: message,
      message,
      data: { code: 400, message },
    })
  }

  try {
    await $fetch('/auth/forgot-password', {
      baseURL: backendBase(event),
      method: 'POST',
      body: { email: body.email, locale: body.locale },
    })
  } catch (err) {
    throw toBackendError(err)
  }

  return { sent: true }
})
