/**
 * Username availability check for the registration form, checked once the
 * field's `change` fires (see web/app/pages/register.vue). Proxies the Go
 * API's public `GET /users/username-available` — same "public, no session"
 * shape as register.post.ts, not the authenticated backendFetch() used
 * elsewhere in server/api/backend/**.
 */
export default defineEventHandler(async (event) => {
  const { username } = getQuery<{ username?: string }>(event)

  if (!username?.trim()) {
    const message = 'username es obligatorio.'
    throw createError({
      statusCode: 400,
      statusMessage: message,
      message,
      data: { code: 400, message },
    })
  }

  try {
    return await $fetch<{ available: boolean }>('/users/username-available', {
      baseURL: backendBase(event),
      query: { username },
    })
  } catch (err) {
    throw toBackendError(err)
  }
})
