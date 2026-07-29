import type { H3Event } from 'h3'

/**
 * Capa BFF (Backend For Frontend) entre el navegador y la API Go.
 *
 * Diseño de sesión (ver web/README.md):
 *  - `cc_access_token` y `cc_refresh_token` son cookies **httpOnly**: las setea
 *    y las lee solo Nitro, el JS del navegador nunca las ve. Un XSS ya no puede
 *    robar los tokens.
 *  - `cc_session` es una cookie NO httpOnly que solo contiene el marcador "1".
 *    No es un credencial: sirve para que el middleware de rutas sepa, tanto en
 *    SSR como en el cliente, si hay sesión sin tener que pegarle a la API.
 *  - El navegador nunca habla con la API Go directamente: todo pasa por
 *    `/api/auth/*` y por el proxy `/api/backend/**`, que inyectan el Bearer
 *    desde la cookie httpOnly.
 */

export interface AuthUser {
  id: string
  username: string
  email: string
  created_at: string
  moxfield_username?: string | null
}

export interface TokenResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: AuthUser
}

export const ACCESS_COOKIE = 'cc_access_token'
export const REFRESH_COOKIE = 'cc_refresh_token'
/** Marcador legible desde JS. NO contiene el token, solo indica "hay sesión". */
export const SESSION_COOKIE = 'cc_session'

/** Debe cubrir al menos REFRESH_TOKEN_TTL del backend (720h = 30 días). */
const REFRESH_MAX_AGE = 60 * 60 * 24 * 30
/** Fallback si el backend no informa expires_in. */
const DEFAULT_ACCESS_MAX_AGE = 15 * 60

/** URL base de la API Go vista desde el proceso Nitro (ver nuxt.config.ts). */
export function backendBase(event: H3Event): string {
  const config = useRuntimeConfig(event)
  return config.apiBase || config.public.apiBase
}

function baseCookieOptions(event: H3Event) {
  return {
    path: '/',
    sameSite: 'lax' as const,
    // En dev y en Docker Compose se sirve por http://localhost, donde `secure`
    // haría que el navegador descarte la cookie.
    secure: getRequestProtocol(event) === 'https',
  }
}

export function setSessionCookies(event: H3Event, data: TokenResponse) {
  const base = baseCookieOptions(event)
  setCookie(event, ACCESS_COOKIE, data.access_token, {
    ...base,
    httpOnly: true,
    maxAge: data.expires_in || DEFAULT_ACCESS_MAX_AGE,
  })
  setCookie(event, REFRESH_COOKIE, data.refresh_token, {
    ...base,
    httpOnly: true,
    maxAge: REFRESH_MAX_AGE,
  })
  setCookie(event, SESSION_COOKIE, '1', {
    ...base,
    httpOnly: false,
    maxAge: REFRESH_MAX_AGE,
  })
}

export function clearSessionCookies(event: H3Event) {
  const base = baseCookieOptions(event)
  for (const name of [ACCESS_COOKIE, REFRESH_COOKIE, SESSION_COOKIE]) {
    deleteCookie(event, name, base)
  }
}

/** Traduce un error de ofetch contra la API Go a un error de h3 equivalente. */
export function toBackendError(err: unknown) {
  const e = err as {
    status?: number
    statusCode?: number
    statusMessage?: string
    data?: { code?: number; message?: string }
  }
  const statusCode = e?.status ?? e?.statusCode ?? 502
  const message =
    e?.data?.message ?? e?.statusMessage ?? 'No se pudo contactar con la API.'

  return createError({
    statusCode,
    statusMessage: message,
    message,
    data: { code: statusCode, message },
  })
}

function statusOf(err: unknown): number | undefined {
  const e = err as { status?: number; statusCode?: number }
  return e?.status ?? e?.statusCode
}

/**
 * Refrescos en vuelo, indexados por refresh token.
 *
 * El backend **rota** el refresh token (revoca el anterior en cada /auth/refresh),
 * así que si dos requests paralelos reciben 401 a la vez y ambos refrescan, el
 * segundo fallaría y cerraría la sesión. Nitro es un único proceso Node, así
 * que alcanza con deduplicar la llamada en memoria.
 */
const inFlightRefresh = new Map<string, Promise<TokenResponse | null>>()

async function callRefresh(
  event: H3Event,
  refreshToken: string,
): Promise<TokenResponse | null> {
  const existing = inFlightRefresh.get(refreshToken)
  if (existing) return existing

  const pending = $fetch<TokenResponse>('/auth/refresh', {
    baseURL: backendBase(event),
    method: 'POST',
    body: { refresh_token: refreshToken },
  }).catch(() => null)

  inFlightRefresh.set(refreshToken, pending)
  try {
    return await pending
  } finally {
    inFlightRefresh.delete(refreshToken)
  }
}

/**
 * Canjea el refresh token por un access token nuevo y actualiza las cookies.
 * Devuelve null (y limpia la sesión) si el refresh token ya no sirve.
 */
export async function refreshSession(event: H3Event): Promise<string | null> {
  const refreshToken = getCookie(event, REFRESH_COOKIE)
  if (!refreshToken) {
    clearSessionCookies(event)
    return null
  }

  const data = await callRefresh(event, refreshToken)
  if (!data) {
    clearSessionCookies(event)
    return null
  }

  setSessionCookies(event, data)
  return data.access_token
}

function sessionExpired() {
  return createError({
    statusCode: 401,
    statusMessage: 'Sesión expirada',
    message: 'Sesión expirada',
    data: { code: 401, message: 'Sesión expirada' },
  })
}

export interface BackendFetchOptions {
  method?: string
  body?: unknown
  query?: Record<string, unknown>
  headers?: Record<string, string>
}

/**
 * Llama a la API Go con el access token de la cookie httpOnly. Si la API
 * responde 401 (access token expirado), refresca una única vez y reintenta.
 * Mismo espíritu que el `AuthAuthenticator` de OkHttp en el cliente Android.
 */
export async function backendFetch<T>(
  event: H3Event,
  path: string,
  options: BackendFetchOptions = {},
): Promise<T> {
  const call = (token: string) =>
    $fetch(path, {
      baseURL: backendBase(event),
      method: (options.method ?? 'GET') as never,
      body: options.body as never,
      query: options.query,
      headers: { ...options.headers, Authorization: `Bearer ${token}` },
    }) as Promise<T>

  let token = getCookie(event, ACCESS_COOKIE)

  // Sin access token (la cookie caducó antes que el refresh token): refrescamos
  // de entrada en vez de gastar un request que sabemos que va a dar 401.
  if (!token) {
    token = (await refreshSession(event)) ?? undefined
    if (!token) throw sessionExpired()
  }

  try {
    return await call(token)
  } catch (err) {
    if (statusOf(err) !== 401) throw toBackendError(err)

    const refreshed = await refreshSession(event)
    if (!refreshed) throw sessionExpired()

    try {
      return await call(refreshed)
    } catch (retryErr) {
      throw toBackendError(retryErr)
    }
  }
}
