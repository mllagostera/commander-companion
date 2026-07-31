/**
 * Resends the verification email. A thin proxy to the Go API's
 * `POST /auth/resend-verification`, which never reveals whether the email exists or is
 * already verified (see internal/users/service.go: ResendVerification) — this endpoint
 * responds success in every case.
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{ email?: string }>(event)

  if (!body?.email) {
    const message = 'Falta el email.'
    throw createError({
      statusCode: 400,
      statusMessage: message,
      message,
      data: { code: 400, message },
    })
  }

  try {
    await $fetch('/auth/resend-verification', {
      baseURL: backendBase(event),
      method: 'POST',
      body: { email: body.email },
    })
  } catch (err) {
    throw toBackendError(err)
  }

  return { sent: true }
})
