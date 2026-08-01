/**
 * Authenticated proxy to the Go API.
 *
 * The browser calls `/api/backend/decks`, `/api/backend/statistics/user`,
 * etc.; Nitro forwards the request to the API adding the `Authorization: Bearer`
 * that comes from the httpOnly cookie, and transparently renews the access
 * token if it expired (see `backendFetch`).
 *
 * The `/auth/*` endpoints are deliberately excluded: they go through
 * `/api/auth/*`, which are the only ones that touch session cookies. This way no
 * path from JS can make a token come back to the browser. The match is done against
 * the lowercased path (`/Auth/login` blocked the same as `/auth/login`): the Go
 * backend's router is case-insensitive by default, so comparing case-sensitively
 * here would let a differently-cased request slip through this filter and still
 * resolve to the real login/refresh handler on the other side.
 */
const BLOCKED_PREFIXES = ['auth/']

const METHODS_WITHOUT_BODY = new Set(['GET', 'HEAD', 'DELETE', 'OPTIONS'])

export default defineEventHandler(async (event) => {
  const raw = getRouterParam(event, 'path') ?? ''
  // Normalizes `//a/../b` and prevents escaping the API prefix.
  const path = raw.split('/').filter((s) => s && s !== '.' && s !== '..').join('/')

  if (!path) {
    throw createError({ statusCode: 404, statusMessage: 'Not Found' })
  }

  const lowerPath = path.toLowerCase()
  if (BLOCKED_PREFIXES.some((prefix) => lowerPath.startsWith(prefix))) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: 'Not Found',
      data: { code: 404, message: 'Not Found' },
    })
  }

  const method = event.method.toUpperCase()
  const body = METHODS_WITHOUT_BODY.has(method)
    ? undefined
    : await readBody(event).catch(() => undefined)

  return await backendFetch(event, `/${path}`, {
    method,
    body,
    query: getQuery(event),
  })
})
