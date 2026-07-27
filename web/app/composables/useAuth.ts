export interface AuthUser {
  id: string
  username: string
  email: string
  created_at: string
}

interface TokenResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  user: AuthUser
}

const COOKIE_OPTS = { sameSite: 'lax' as const, maxAge: 60 * 60 * 24 * 30 }

export function useAuth() {
  const config = useRuntimeConfig()
  // En SSR (Docker Compose) el servidor debe llamar a la API por su hostname
  // interno; el navegador necesita la URL pública. Ver nuxt.config.ts.
  const apiBase = import.meta.server ? config.apiBase : config.public.apiBase
  const accessToken = useCookie<string | null>('cc_access_token', COOKIE_OPTS)
  const refreshToken = useCookie<string | null>('cc_refresh_token', COOKIE_OPTS)
  const user = useState<AuthUser | null>('auth-user', () => null)

  function applyTokenResponse(data: TokenResponse) {
    accessToken.value = data.access_token
    refreshToken.value = data.refresh_token
    user.value = data.user
  }

  function clearSession() {
    accessToken.value = null
    refreshToken.value = null
    user.value = null
  }

  async function login(email: string, password: string) {
    const data = await $fetch<TokenResponse>('/auth/login', {
      baseURL: apiBase,
      method: 'POST',
      body: { email, password },
    })
    applyTokenResponse(data)
  }

  async function loginWithGoogle(idToken: string) {
    const data = await $fetch<TokenResponse>('/auth/google', {
      baseURL: apiBase,
      method: 'POST',
      body: { id_token: idToken },
    })
    applyTokenResponse(data)
  }

  async function fetchMe() {
    if (!accessToken.value) {
      user.value = null
      return null
    }
    try {
      const data = await $fetch<AuthUser>('/auth/me', {
        baseURL: apiBase,
        headers: { Authorization: `Bearer ${accessToken.value}` },
      })
      user.value = data
      return data
    } catch {
      clearSession()
      return null
    }
  }

  async function logout() {
    if (refreshToken.value) {
      try {
        await $fetch('/auth/logout', {
          baseURL: apiBase,
          method: 'POST',
          body: { refresh_token: refreshToken.value },
        })
      } catch {
        // Best-effort: igual limpiamos la sesión local aunque el backend falle.
      }
    }
    clearSession()
  }

  return {
    user,
    accessToken,
    isAuthenticated: computed(() => !!accessToken.value),
    login,
    loginWithGoogle,
    logout,
    fetchMe,
  }
}
