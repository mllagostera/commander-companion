import type { TokenResponse } from '../../utils/backend'

/**
 * Login (o alta) con Google. El id_token de Google Identity Services viaja del
 * navegador a Nitro y de Nitro a la API Go; los tokens de sesión que devuelve
 * la API quedan en cookies httpOnly.
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{ id_token?: string }>(event)

  if (!body?.id_token) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Falta el id_token de Google.',
      message: 'Falta el id_token de Google.',
      data: { code: 400, message: 'Falta el id_token de Google.' },
    })
  }

  try {
    const data = await $fetch<TokenResponse>('/auth/google', {
      baseURL: backendBase(event),
      method: 'POST',
      body: { id_token: body.id_token },
    })
    setSessionCookies(event, data)
    return { user: data.user }
  } catch (err) {
    clearSessionCookies(event)
    throw toBackendError(err)
  }
})
