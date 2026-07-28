import type { AuthUser } from '../../utils/backend'

/**
 * Alta de usuario nuevo.
 *
 * `POST /auth/register` de la API Go crea la cuenta sin verificar el email y manda un
 * mail de confirmación (ver internal/users/service.go: RegisterUser). Ya no encadenamos
 * un login automático: hasta no verificar, `POST /auth/login` responde 403 (ver
 * login.post.ts), así que dejar la sesión iniciada acá solo produciría una sesión que
 * el resto de la API rechazaría igual. El cliente muestra un "revisá tu email" en vez
 * de navegar al dashboard.
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

  try {
    await $fetch<AuthUser>('/auth/register', {
      baseURL: backendBase(event),
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

  return { email: body.email }
})
