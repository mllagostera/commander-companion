import type { NitroFetchArgs } from './useNitroFetch'

/**
 * Cliente de la API Go para las pantallas.
 *
 * Nunca llama a la API directamente: pasa por el proxy `/api/backend/**` de
 * Nitro, que es quien conoce el access token (cookie httpOnly) y quien lo
 * renueva contra `POST /auth/refresh` si venció.
 *
 * Un 401 acá significa que ya no se pudo ni refrescar (refresh token revocado
 * o expirado): cerramos la sesión local y mandamos al login.
 */
export function useApi() {
  const nitroFetch = useNitroFetch()
  const { resetSession } = useAuth()

  async function apiFetch<T>(
    path: string,
    options: NitroFetchArgs = {},
  ): Promise<T> {
    try {
      return await nitroFetch<T>(`/api/backend${path}`, options)
    } catch (err) {
      if (apiErrorStatus(err) === 401) {
        resetSession()
        if (import.meta.client) {
          await navigateTo('/login')
        }
      }
      throw err
    }
  }

  return { apiFetch }
}

/** Status HTTP de un error de ofetch/h3, sin importar de qué capa venga. */
export function apiErrorStatus(err: unknown): number | undefined {
  const e = err as { status?: number; statusCode?: number }
  return e?.status ?? e?.statusCode
}

/** Mensaje legible de un error de la API, con fallback. */
export function apiErrorMessage(err: unknown, fallback: string): string {
  const e = err as {
    data?: { message?: string }
    statusMessage?: string
    message?: string
  }
  return e?.data?.message || e?.statusMessage || fallback
}
