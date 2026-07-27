/**
 * Proxy autenticado hacia la API Go.
 *
 * El navegador llama a `/api/backend/decks`, `/api/backend/statistics/user`,
 * etc.; Nitro reenvía la request a la API agregando el `Authorization: Bearer`
 * que sale de la cookie httpOnly, y renueva el access token de forma
 * transparente si venció (ver `backendFetch`).
 *
 * Los endpoints `/auth/*` quedan deliberadamente fuera: van por
 * `/api/auth/*`, que son los únicos que tocan cookies de sesión. Así ningún
 * camino desde JS puede hacer que un token vuelva al navegador.
 */
const BLOCKED_PREFIXES = ['auth/']

const METHODS_WITHOUT_BODY = new Set(['GET', 'HEAD', 'DELETE', 'OPTIONS'])

export default defineEventHandler(async (event) => {
  const raw = getRouterParam(event, 'path') ?? ''
  // Normaliza `//a/../b` y evita salir del prefijo de la API.
  const path = raw.split('/').filter((s) => s && s !== '.' && s !== '..').join('/')

  if (!path) {
    throw createError({ statusCode: 404, statusMessage: 'Not Found' })
  }

  if (BLOCKED_PREFIXES.some((prefix) => path.startsWith(prefix))) {
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
