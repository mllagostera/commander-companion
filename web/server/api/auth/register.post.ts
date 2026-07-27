import type { AuthUser, TokenResponse } from '../../utils/backend'

/**
 * Alta de usuario nuevo.
 *
 * `POST /auth/register` de la API Go devuelve el User creado pero **no**
 * tokens, así que encadenamos un `POST /auth/login` con las mismas
 * credenciales para dejar la sesión iniciada y evitar que el usuario tenga que
 * escribir su password dos veces seguidas.
 */
export default defineEventHandler(async (event) => {
  const body = await readBody<{
    username?: string
    email?: string
    password?: string
  }>(event)

  if (!body?.username || !body?.email || !body?.password) {
    const message = 'Usuario, email y contraseña son obligatorios.'
    throw createError({
      statusCode: 400,
      statusMessage: message,
      message,
      data: { code: 400, message },
    })
  }

  const baseURL = backendBase(event)

  try {
    await $fetch<AuthUser>('/auth/register', {
      baseURL,
      method: 'POST',
      body: {
        username: body.username,
        email: body.email,
        password: body.password,
      },
    })
  } catch (err) {
    throw toBackendError(err)
  }

  try {
    const data = await $fetch<TokenResponse>('/auth/login', {
      baseURL,
      method: 'POST',
      body: { email: body.email, password: body.password },
    })
    setSessionCookies(event, data)
    return { user: data.user }
  } catch (err) {
    // La cuenta quedó creada pero el auto-login falló: el usuario puede
    // entrar a mano desde /login.
    clearSessionCookies(event)
    throw toBackendError(err)
  }
})
