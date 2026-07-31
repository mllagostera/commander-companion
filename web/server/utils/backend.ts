import type { H3Event } from 'h3'

/**
 * BFF (Backend For Frontend) layer between the browser and the Go API.
 *
 * Session design (see web/README.md):
 *  - `cc_access_token` and `cc_refresh_token` are **httpOnly** cookies: only Nitro
 *    sets and reads them, the browser's JS never sees them. An XSS can no longer
 *    steal the tokens.
 *  - `cc_session` is a NON-httpOnly cookie that only contains the "1" marker.
 *    It's not a credential: it lets the route middleware know, both in
 *    SSR and on the client, whether there's a session without having to hit the API.
 *  - The browser never talks to the Go API directly: everything goes through
 *    `/api/auth/*` and the `/api/backend/**` proxy, which inject the Bearer
 *    from the httpOnly cookie.
 */

export interface AuthUser {
  id: string
  username: string
  email: string
  created_at: string
  moxfield_username?: string | null
  /** false = account created via Google Sign-In, with no password of its own. */
  has_password: boolean
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
/** Marker readable from JS. Does NOT contain the token, only signals "there's a session". */
export const SESSION_COOKIE = 'cc_session'

/** Must cover at least the backend's REFRESH_TOKEN_TTL (720h = 30 days). */
const REFRESH_MAX_AGE = 60 * 60 * 24 * 30
/** Fallback if the backend doesn't report expires_in. */
const DEFAULT_ACCESS_MAX_AGE = 15 * 60

/** Base URL of the Go API as seen from the Nitro process (see nuxt.config.ts). */
export function backendBase(event: H3Event): string {
  const config = useRuntimeConfig(event)
  return config.apiBase || config.public.apiBase
}

function baseCookieOptions(event: H3Event) {
  return {
    path: '/',
    sameSite: 'lax' as const,
    // In dev and Docker Compose it's served over http://localhost, where `secure`
    // would make the browser discard the cookie.
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

/** Translates an ofetch error against the Go API into an equivalent h3 error. */
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
 * In-flight refreshes, indexed by refresh token.
 *
 * The backend **rotates** the refresh token (revokes the previous one on every /auth/refresh),
 * so if two parallel requests get a 401 at the same time and both refresh, the
 * second one would fail and end the session. Nitro is a single Node process, so
 * it's enough to deduplicate the call in memory.
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
 * Exchanges the refresh token for a new access token and updates the cookies.
 * Returns null (and clears the session) if the refresh token is no longer valid.
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
 * Calls the Go API with the access token from the httpOnly cookie. If the API
 * responds with 401 (expired access token), refreshes once and retries.
 * Same spirit as the `AuthAuthenticator` from OkHttp in the Android client.
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

  // No access token (the cookie expired before the refresh token): we refresh
  // upfront instead of spending a request we already know will return 401.
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
