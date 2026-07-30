export interface AuthUser {
  id: string
  username: string
  email: string
  created_at: string
  moxfield_username?: string | null
  /** false = cuenta creada vía Google Sign-In, sin password propio. */
  has_password: boolean
}

interface SessionResponse {
  user: AuthUser | null
}

/**
 * Sesión del cliente web.
 *
 * Los tokens **no** viven acá: son cookies httpOnly que solo maneja Nitro
 * (`web/server/api/auth/*`, ver `web/server/utils/backend.ts`). Desde el
 * navegador solo se ve `cc_session`, un marcador sin valor sensible que sirve
 * para saber si hay sesión sin pegarle a la API — lo usa el middleware de
 * rutas, que corre tanto en SSR como en el cliente.
 */
export function useAuth() {
  const user = useState<AuthUser | null>('auth-user', () => null)
  const sessionMarker = useCookie<string | null>('cc_session', {
    path: '/',
    sameSite: 'lax',
  })
  const nitroFetch = useNitroFetch()

  // El estado vive en `useState`, no en el ref de la cookie: en SSR cada
  // llamada a `useCookie` crea un ref nuevo leído de la request original, así
  // que invalidar la sesión en un plugin no se vería después en el middleware.
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
   * Registra la cuenta. No deja sesión iniciada: hasta no verificar el email,
   * `login()` responde 403 (ver server/api/auth/login.post.ts), así que la pantalla de
   * registro muestra un "revisa tu email" en vez de navegar al dashboard.
   */
  async function register(username: string, email: string, password: string) {
    await nitroFetch('/api/auth/register', {
      method: 'POST',
      body: { username, email, password },
    })
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
      // Best-effort: igual limpiamos la sesión local aunque el backend falle.
    }
    resetSession()
  }

  /**
   * Rehidrata el usuario a partir de las cookies httpOnly. Nunca tira 401: si
   * no hay sesión válida devuelve null (Nitro ya limpió las cookies).
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
    verifyEmail,
    resendVerification,
    loginWithGoogle,
    logout,
    fetchSession,
    resetSession,
  }
}
