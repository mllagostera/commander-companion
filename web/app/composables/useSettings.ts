import type { AuthUser } from './useAuth'
import type { MoxfieldImportJob } from '~/types/api'

/**
 * Ajustes de la cuenta propia: cambio de password y vínculo con Moxfield
 * (username de perfil + import masivo en background). Separado de `useAuth`
 * porque estos endpoints van por el proxy autenticado (`useApi`/`/api/backend`),
 * no por `/api/auth/*` (ver el comentario de `useApi.ts`).
 */
export function useSettings() {
  const { apiFetch } = useApi()
  const { user } = useAuth()

  function requireUserId(): string {
    const id = user.value?.id
    if (!id) throw new Error('No hay una sesión activa.')
    return id
  }

  /** Vincula (o cambia) el username de Moxfield del perfil propio. */
  function updateMoxfieldUsername(moxfieldUsername: string) {
    return apiFetch<AuthUser>(`/users/${requireUserId()}`, {
      method: 'PATCH',
      body: { moxfield_username: moxfieldUsername.trim() },
    })
  }

  /** 204 sin body si el password actual es correcto y el nuevo se aplicó. */
  function changePassword(currentPassword: string, newPassword: string) {
    return apiFetch<null>(`/users/${requireUserId()}/password`, {
      method: 'POST',
      body: { current_password: currentPassword, new_password: newPassword },
    })
  }

  /** Dispara el import masivo de todos los decks públicos del moxfield_username vinculado. */
  function startMoxfieldImport() {
    return apiFetch<MoxfieldImportJob>('/moxfield-import', { method: 'POST' })
  }

  function getMoxfieldImportStatus(jobId: string) {
    return apiFetch<MoxfieldImportJob>(`/moxfield-import/${jobId}`)
  }

  return {
    updateMoxfieldUsername,
    changePassword,
    startMoxfieldImport,
    getMoxfieldImportStatus,
  }
}

/**
 * Traduce los errores de POST /users/{id}/password. 401 cubre dos casos de
 * dominio distintos (password actual incorrecto y cuenta de Google sin
 * password propio, ver internal/users/service.go): se distinguen por el
 * contenido del mensaje, mismo criterio que moxfieldImportError con el 400
 * de "sin comandante".
 */
export function changePasswordError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 400:
      return 'La contraseña nueva debe tener al menos 8 caracteres.'
    case 401:
      return apiErrorMessage(err, '').toLowerCase().includes('google')
        ? 'Esta cuenta inició sesión con Google y no tiene contraseña propia.'
        : 'La contraseña actual no es correcta.'
    default:
      return apiErrorMessage(err, 'No se pudo cambiar la contraseña.')
  }
}

/** Traduce los errores de PATCH /users/{id} al actualizar moxfield_username. */
export function updateMoxfieldUsernameError(err: unknown): string {
  return apiErrorMessage(err, 'No se pudo guardar el usuario de Moxfield.')
}

/**
 * Traduce los errores de POST /moxfield-import. 501 es el estado esperado hoy:
 * MoxfieldClient.ListDecksByUsername es un stub (ver internal/moxfieldimport,
 * doc del paquete) hasta confirmar el endpoint real de Moxfield.
 */
export function startMoxfieldImportError(err: unknown): string {
  switch (apiErrorStatus(err)) {
    case 400:
      return 'Primero guardá tu usuario de Moxfield.'
    case 409:
      return 'Ya hay una importación en curso para tu cuenta.'
    case 501:
      return 'La importación masiva de Moxfield todavía no está disponible en el servidor.'
    default:
      return apiErrorMessage(err, 'No se pudo iniciar la importación.')
  }
}
