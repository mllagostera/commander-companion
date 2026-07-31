import type { H3Event } from 'h3'

/** Supported options when talking to Nitro's own endpoints. */
export interface NitroFetchArgs {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: Record<string, unknown>
  query?: Record<string, unknown>
  headers?: Record<string, string>
}

/** Key of the per-request "cookie jar" that lives in the H3Event context. */
const JAR_KEY = '__ccCookieJar'

type CookieJar = Map<string, string>

/**
 * `$fetch` to Nitro's own endpoints (`/api/auth/*`,
 * `/api/backend/**`), usable interchangeably in SSR and on the client.
 *
 * In the browser nothing special is needed: cookies come and go on their own.
 * In SSR, on the other hand, every internal call runs in its own H3Event, so
 * this composable keeps a per-request cookie jar so that:
 *
 *  1. The call carries the session cookies (without them Nitro wouldn't see the
 *     httpOnly session). `useRequestFetch` isn't used because the fetch bound to
 *     the event that it returns in SSR doesn't expose `.raw`, and without `.raw` the
 *     response's `Set-Cookie` headers can't be read.
 *  2. The `Set-Cookie` headers the handler emits reach the browser (they have to
 *     be copied by hand to the response that's actually sent).
 *  3. **Subsequent calls in the same render see those new cookies.**
 *     This matters because the backend rotates the refresh token: if the session plugin
 *     refreshes and the page then requests data with the original cookie
 *     header, it would go with an already-revoked refresh token and the session would drop.
 */
export function useNitroFetch() {
  const event = import.meta.server ? useRequestEvent() : undefined
  const initialCookie = import.meta.server
    ? (useRequestHeaders(['cookie']).cookie ?? '')
    : ''

  return async function nitroFetch<T>(
    path: string,
    options: NitroFetchArgs = {},
  ): Promise<T> {
    const headers: Record<string, string> = { ...options.headers }

    if (import.meta.server && event) {
      const cookie = serializeJar(getJar(event, initialCookie))
      if (cookie) headers.cookie = cookie
    }

    const response = await $fetch.raw<T>(path, { ...options, headers })

    if (import.meta.server && event) {
      const setCookies =
        typeof response.headers.getSetCookie === 'function'
          ? response.headers.getSetCookie()
          : []

      if (setCookies.length) {
        const jar = getJar(event, initialCookie)
        const { appendResponseHeader } = await import('h3')
        for (const cookie of setCookies) {
          appendResponseHeader(event, 'set-cookie', cookie)
          applySetCookie(jar, cookie)
        }
      }
    }

    return response._data as T
  }
}

function getJar(event: H3Event, initialCookie: string): CookieJar {
  const context = event.context as Record<string, unknown>
  let jar = context[JAR_KEY] as CookieJar | undefined

  if (!jar) {
    jar = new Map()
    for (const part of initialCookie.split(';')) {
      const separator = part.indexOf('=')
      if (separator < 0) continue
      jar.set(part.slice(0, separator).trim(), part.slice(separator + 1).trim())
    }
    context[JAR_KEY] = jar
  }

  return jar
}

/** Applies a `Set-Cookie` header to the jar (add, change, or delete). */
function applySetCookie(jar: CookieJar, setCookie: string) {
  const [pair = '', ...attributes] = setCookie.split(';')
  const separator = pair.indexOf('=')
  if (separator < 0) return

  const name = pair.slice(0, separator).trim()
  const value = pair.slice(separator + 1).trim()

  const isExpired = attributes.some((attribute) => {
    const [rawKey = '', rawValue = ''] = attribute.split('=')
    const key = rawKey.trim().toLowerCase()
    if (key === 'max-age') return Number(rawValue) <= 0
    if (key === 'expires') return new Date(rawValue.trim()).getTime() <= Date.now()
    return false
  })

  if (!value || isExpired) jar.delete(name)
  else jar.set(name, value)
}

function serializeJar(jar: CookieJar): string {
  return [...jar].map(([name, value]) => `${name}=${value}`).join('; ')
}
