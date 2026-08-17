export interface AuthUser {
  id: string
  username: string
  email: string
  created_at: string
  moxfield_username?: string | null
  /** false = account created via Google Sign-In, with no password of its own. */
  has_password: boolean
  /** Grants access to the admin dashboard (/admin/*). See ADR-0018. */
  is_admin: boolean
  /** False = an admin deactivated this account. In practice always true here: a
   * deactivated account never reaches this far (login/refresh reject it first). */
  is_active: boolean
}

interface SessionResponse {
  user: AuthUser | null
}

/**
 * Web client session.
 *
 * The tokens do **not** live here: they're httpOnly cookies that only Nitro
 * handles (`web/server/api/auth/*`, see `web/server/utils/backend.ts`). From the
 * browser only `cc_session` is visible, a marker with no sensitive value used
 * to know whether there's a session without hitting the API — it's used by the
 * route middleware, which runs in both SSR and the client.
 */
export function useAuth() {
  const user = useState<AuthUser | null>('auth-user', () => null)
  const sessionMarker = useCookie<string | null>('cc_session', {
    path: '/',
    sameSite: 'lax',
  })
  const nitroFetch = useNitroFetch()

  // The state lives in `useState`, not in the cookie's ref: in SSR every
  // call to `useCookie` creates a new ref read from the original request, so
  // invalidating the session in a plugin wouldn't be reflected later in the middleware.
  const hasSession = useState<boolean>(
    'auth-has-session',
    () => !!sessionMarker.value,
  )

  const isAuthenticated = computed(() => hasSession.value)

  function resetSession() {
    user.value = null
    hasSession.value = false
    sessionMarker.value = null
  }

  function applySession(data: SessionResponse) {
    user.value = data.user
    hasSession.value = !!data.user
    sessionMarker.value = data.user ? '1' : null
    return data.user
  }

  async function login(email: string, password: string) {
    return applySession(
      await nitroFetch<SessionResponse>('/api/auth/login', {
        method: 'POST',
        body: { email, password },
      }),
    )
  }

  /**
   * Registers the account. Doesn't leave a session started: until the email is verified,
   * `login()` responds 403 (see server/api/auth/login.post.ts), so the registration
   * screen shows a "check your email" instead of navigating to the dashboard.
   */
  async function register(username: string, email: string, password: string) {
    await nitroFetch('/api/auth/register', {
      method: 'POST',
      body: { username, email, password },
    })
  }

  /**
   * Checks whether a username is free to register. Public (no session needed),
   * used by the registration form on the field's `change` event, before submitting.
   */
  async function checkUsernameAvailable(username: string) {
    return (
      await nitroFetch<{ available: boolean }>('/api/auth/username-available', {
        query: { username },
      })
    ).available
  }

  async function verifyEmail(token: string) {
    await nitroFetch('/api/auth/verify-email', {
      method: 'POST',
      body: { token },
    })
  }

  async function resendVerification(email: string) {
    await nitroFetch('/api/auth/resend-verification', {
      method: 'POST',
      body: { email },
    })
  }

  async function loginWithGoogle(idToken: string) {
    return applySession(
      await nitroFetch<SessionResponse>('/api/auth/google', {
        method: 'POST',
        body: { id_token: idToken },
      }),
    )
  }

  async function logout() {
    try {
      await nitroFetch('/api/auth/logout', { method: 'POST' })
    } catch {
      // Best-effort: we still clear the local session even if the backend fails.
    }
    resetSession()
  }

  /**
   * Rehydrates the user from the httpOnly cookies. Never throws 401: if
   * there's no valid session it returns null (Nitro has already cleared the cookies).
   */
  async function fetchSession() {
    try {
      return applySession(await nitroFetch<SessionResponse>('/api/auth/session'))
    } catch {
      resetSession()
      return null
    }
  }

  return {
    user,
    isAuthenticated,
    login,
    register,
    checkUsernameAvailable,
    verifyEmail,
    resendVerification,
    loginWithGoogle,
    logout,
    fetchSession,
    resetSession,
  }
}
