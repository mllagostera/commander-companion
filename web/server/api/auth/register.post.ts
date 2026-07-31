import type { AuthUser } from '../../utils/backend'

/**
 * New user sign-up.
 *
 * The Go API's `POST /auth/register` creates the account without verifying the email and
 * sends a confirmation email (see internal/users/service.go: RegisterUser). We no longer
 * chain an automatic login: until verified, `POST /auth/login` responds 403 (see
 * login.post.ts), so leaving a session started here would just produce a session that
 * the rest of the API would reject anyway. The client shows a "check your email" instead
 * of navigating to the dashboard.
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
