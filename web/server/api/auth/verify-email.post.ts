/**
 * Confirma la cuenta a partir del token que llegó por mail. Proxy fino a
 * `POST /auth/verify-email` de la API Go: el token viaja en el body (no en una query
 * string) porque el mail linkea a la página `/verify-email?token=...` de este mismo
 * cliente, que es quien hace este POST (ver internal/users/handler.go: VerifyEmail).
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
