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

function notFound() {
  return createError({
    statusCode: 404,
    statusMessage: 'Not Found',
    message: 'Not Found',
    data: { code: 404, message: 'Not Found' },
  })
}

/**
 * Turns the raw catch-all param into a path that can't leave the API prefix.
 *
 * h3 hands the param over still percent-encoded, and the URL parser behind
 * `$fetch` treats `%2e%2e` exactly like `..` — so filtering only the literal
 * dots let `/api/backend/%2e%2e/v1/auth/me` reach the blocked `/auth/*`
 * endpoints (and anything outside `/api/v1`). Each segment is decoded first,
 * dot segments and segments hiding a separator are dropped or rejected, and the
 * survivors are re-encoded, so what the filter checks is what the API receives.
 * Returns null when a segment can't be decoded or would smuggle a separator.
 */
function normalizeProxyPath(raw: string): string | null {
  const segments: string[] = []
  for (const encoded of raw.split('/')) {
    let segment: string
    try {
      segment = decodeURIComponent(encoded)
    } catch {
      return null
    }
    if (!segment || segment === '.' || segment === '..') continue
    if (/[/\\%]/.test(segment)) return null
    segments.push(segment)
  }
  return segments.map(encodeURIComponent).join('/')
}

export default defineEventHandler(async (event) => {
  const path = normalizeProxyPath(getRouterParam(event, 'path') ?? '')
  if (!path) throw notFound()

  const lowerPath = path.toLowerCase()
  if (BLOCKED_PREFIXES.some((prefix) => lowerPath.startsWith(prefix))) {
    throw notFound()
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
