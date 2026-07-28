/**
 * Reenvía el mail de verificación. Proxy fino a `POST /auth/resend-verification` de la
 * API Go, que nunca revela si el email existe o ya está verificado (ver
 * internal/users/service.go: ResendVerification) — este endpoint responde éxito igual
 * en todos los casos.
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
