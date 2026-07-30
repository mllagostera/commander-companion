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
    if (!id) throw new Error(useI18n().t('errors.auth.noActiveSession'))
    return id
  }

  /** Vincula (o cambia) el username de Moxfield del perfil propio. */
  function updateMoxfieldUsername(moxfieldUsername: string) {
    return apiFetch<AuthUser>(`/users/${requireUserId()}`, {
      method: 'PATCH',
      body: { moxfield_username: moxfieldUsername.trim() },
    })
  }

  /** Cambia el username de login/perfil propio (distinto del de Moxfield). 409 si ya está en uso. */
  function updateUsername(username: string) {
    return apiFetch<AuthUser>(`/users/${requireUserId()}`, {
      method: 'PATCH',
      body: { username: username.trim() },
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
    updateUsername,
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
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.changePassword.tooShort')
    case 401:
      return apiErrorMessage(err, '').toLowerCase().includes('google')
        ? t('errors.changePassword.googleAccount')
        : t('errors.changePassword.incorrect')
    default:
      return apiErrorMessage(err, t('errors.changePassword.generic'))
  }
}

/** Traduce los errores de PATCH /users/{id} al actualizar moxfield_username. */
export function updateMoxfieldUsernameError(err: unknown): string {
  return apiErrorMessage(err, useI18n().t('errors.updateMoxfieldUsername.generic'))
}

/** Traduce los errores de PATCH /users/{id} al actualizar el username de login. */
export function updateUsernameError(err: unknown): string {
  const { t } = useI18n()
  if (apiErrorStatus(err) === 409) return t('errors.updateUsername.taken')
  return apiErrorMessage(err, t('errors.updateUsername.generic'))
}

/**
 * Traduce los errores de POST /moxfield-import. 501 es el estado esperado hoy:
 * MoxfieldClient.ListDecksByUsername es un stub (ver internal/moxfieldimport,
 * doc del paquete) hasta confirmar el endpoint real de Moxfield.
 */
export function startMoxfieldImportError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.startMoxfieldImport.needUsername')
    case 409:
      return t('errors.startMoxfieldImport.inProgress')
    case 501:
      return t('errors.startMoxfieldImport.notAvailable')
    default:
      return apiErrorMessage(err, t('errors.startMoxfieldImport.generic'))
  }
}
