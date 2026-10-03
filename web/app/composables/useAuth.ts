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
/** The active UI locale ("es", "en", "ca"), read before any await so the Nuxt context is still there. */
function currentLocale(): string {
  return useNuxtApp().$i18n.locale.value
}

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
   * screen shows a "check your email" instead of navigating to the dashboard. The active
   * locale goes along so the verification email is sent in the user's language.
   */
  async function register(username: string, email: string, password: string) {
    await nitroFetch('/api/auth/register', {
      method: 'POST',
      body: { username, email, password, locale: currentLocale() },
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
      body: { email, locale: currentLocale() },
    })
  }

  /** Emails a reset link if the address has an account; never says whether it does. */
  async function requestPasswordReset(email: string) {
    await nitroFetch('/api/auth/forgot-password', {
      method: 'POST',
      body: { email, locale: currentLocale() },
    })
  }

  /**
   * Sets a new password from the emailed token. The backend signs the account out
   * everywhere, and the Nitro route drops this browser's cookies, so the local session
   * goes too.
   */
  async function resetPassword(token: string, newPassword: string) {
    await nitroFetch('/api/auth/reset-password', {
      method: 'POST',
      body: { token, new_password: newPassword },
    })
    resetSession()
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
    requestPasswordReset,
    resetPassword,
    loginWithGoogle,
    logout,
    fetchSession,
    resetSession,
  }
}

/*
 * Error translation for the auth pages. The backend's messages are English-only
 * (internal/users/service.go), so they're never shown as-is here: each status the
 * flow can realistically hit maps to a translated message, and anything else falls
 * back to the flow's generic one.
 */

/** Statuses every auth call can get regardless of the flow; undefined when it's flow-specific. */
function commonAuthError(err: unknown): string | undefined {
  const { t } = useNuxtApp().$i18n
  const status = apiErrorStatus(err)
  if (status === 429) return t('errors.auth.tooManyRequests')
  // 502 is what server/utils/backend.ts:toBackendError uses when the API didn't answer.
  if (status === undefined || status === 502 || status === 503) return t('errors.auth.unreachable')
  return undefined
}

/**
 * True when a 403 from login means "email not confirmed" rather than "account
 * deactivated": both are 403, and only the backend message tells them apart
 * (ErrEmailNotConfirmed vs ErrAccountDeactivated).
 */
export function isEmailNotConfirmedError(err: unknown): boolean {
  return apiErrorStatus(err) === 403 && !apiErrorMessage(err, '').toLowerCase().includes('deactivated')
}

/** See ErrInvalidEmail/ErrPasswordTooShort (400) and ErrUserAlreadyExists (409). */
export function registerError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('register.errors.invalidData')
    case 409:
      return t('register.errors.alreadyExists')
    default:
      return commonAuthError(err) ?? t('register.errors.registerFailed')
  }
}

/** See ErrInvalidCredentials/ErrGoogleOnlyAccount (401) and the two 403s in isEmailNotConfirmedError. */
export function loginError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('login.errors.missingFields')
    case 401:
      return apiErrorMessage(err, '').toLowerCase().includes('google')
        ? t('login.errors.googleOnlyAccount')
        : t('login.errors.badCredentials')
    case 403:
      return isEmailNotConfirmedError(err)
        ? t('login.errors.emailNotConfirmed')
        : t('login.errors.accountDeactivated')
    default:
      return commonAuthError(err) ?? t('login.errors.loginFailed')
  }
}

/** See auth.Handler.GoogleLogin: 400 rejected token/unverified email, 403 deactivated, 501 not configured. */
export function googleLoginError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('login.errors.googleRejected')
    case 403:
      return t('login.errors.accountDeactivated')
    case 501:
      return t('login.errors.googleNotConfigured')
    default:
      return commonAuthError(err) ?? t('login.errors.googleFailed')
  }
}

export function resendVerificationError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  return commonAuthError(err) ?? t('login.errors.resendFailed')
}

/** See ErrInvalidVerificationToken (400): the token doesn't exist, was used, or expired. */
export function verifyEmailError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  if (apiErrorStatus(err) === 400) return t('verifyEmail.invalidOrExpired')
  return commonAuthError(err) ?? t('verifyEmail.verifyFailed')
}

export function forgotPasswordError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  return commonAuthError(err) ?? t('forgotPassword.errors.requestFailed')
}

/**
 * True when a reset failed because the link is dead (ErrInvalidPasswordResetToken: unknown,
 * used or expired token). It shares 400 with ErrPasswordTooShort; only the backend message
 * tells them apart.
 */
export function isInvalidResetTokenError(err: unknown): boolean {
  return apiErrorStatus(err) === 400 && apiErrorMessage(err, '').toLowerCase().includes('token')
}

export function resetPasswordError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  if (isInvalidResetTokenError(err)) return t('resetPassword.errors.invalidOrExpired')
  if (apiErrorStatus(err) === 400) return t('resetPassword.errors.passwordTooShort')
  return commonAuthError(err) ?? t('resetPassword.errors.resetFailed')
}
