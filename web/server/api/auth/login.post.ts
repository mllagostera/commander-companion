import type { TokenResponse } from '../../utils/backend'

/**
 * Login con email + password. Los tokens quedan en cookies httpOnly y al
 * navegador solo le vuelve el usuario.
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{ email?: string; password?: string }>(event)

  if (!body?.email || !body?.password) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Email y contraseña son obligatorios.',
      message: 'Email y contraseña son obligatorios.',
      data: { code: 400, message: 'Email y contraseña son obligatorios.' },
    })
  }

  try {
    const data = await $fetch<TokenResponse>('/auth/login', {
      baseURL: backendBase(event),
      method: 'POST',
      body: { email: body.email, password: body.password },
    })
    setSessionCookies(event, data)
    return { user: data.user }
  } catch (err) {
    clearSessionCookies(event)
    throw toBackendError(err)
  }
})
