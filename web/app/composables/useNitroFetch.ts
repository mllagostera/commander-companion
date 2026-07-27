import type { H3Event } from 'h3'

/** Opciones soportadas al hablar con los endpoints propios de Nitro. */
export interface NitroFetchArgs {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: Record<string, unknown>
  query?: Record<string, unknown>
  headers?: Record<string, string>
}

/** Clave del "cookie jar" por request que vive en el contexto del H3Event. */
const JAR_KEY = '__ccCookieJar'

type CookieJar = Map<string, string>

/**
 * `$fetch` hacia los endpoints propios de Nitro (`/api/auth/*`,
 * `/api/backend/**`), usable indistintamente en SSR y en el cliente.
 *
 * En el navegador no hace falta nada especial: las cookies van y vienen solas.
 * En SSR, en cambio, cada llamada interna corre en su propio H3Event, así que
 * este composable mantiene un cookie jar por request para que:
 *
 *  1. La llamada lleve las cookies de sesión (sin ellas Nitro no vería la
 *     sesión httpOnly). No se usa `useRequestFetch` porque el fetch ligado al
 *     evento que devuelve en SSR no expone `.raw`, y sin `.raw` no se pueden
 *     leer los `Set-Cookie` de la respuesta.
 *  2. Los `Set-Cookie` que emita el handler lleguen al navegador (hay que
 *     copiarlos a mano a la respuesta que sí se envía).
 *  3. **Las llamadas siguientes del mismo render vean esas cookies nuevas.**
 *     Importa porque el backend rota el refresh token: si el plugin de sesión
 *     refresca y después la página pide datos con el header de cookies
 *     original, iría con un refresh token ya revocado y la sesión se caería.
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

/** Aplica un header `Set-Cookie` al jar (alta, cambio o borrado). */
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
