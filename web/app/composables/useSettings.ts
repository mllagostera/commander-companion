import type { AuthUser } from './useAuth'
import type { MoxfieldImportJob } from '~/types/api'

/**
 * Own account settings: password change and Moxfield linking
 * (profile username + background bulk import). Kept separate from `useAuth`
 * because these endpoints go through the authenticated proxy (`useApi`/`/api/backend`),
 * not through `/api/auth/*` (see the comment in `useApi.ts`).
 */
export function useSettings() {
  const { apiFetch } = useApi()
  const { user } = useAuth()

  function requireUserId(): string {
    const id = user.value?.id
    if (!id) throw new Error(useI18n().t('errors.auth.noActiveSession'))
    return id
  }

  /** Links (or changes) the profile's own Moxfield username. */
  function updateMoxfieldUsername(moxfieldUsername: string) {
    return apiFetch<AuthUser>(`/users/${requireUserId()}`, {
      method: 'PATCH',
      body: { moxfield_username: moxfieldUsername.trim() },
    })
  }

  /** Changes the own login/profile username (different from the Moxfield one). 409 if already in use. */
  function updateUsername(username: string) {
    return apiFetch<AuthUser>(`/users/${requireUserId()}`, {
      method: 'PATCH',
      body: { username: username.trim() },
    })
  }

  /** 204 with no body if the current password is correct and the new one was applied. */
  function changePassword(currentPassword: string, newPassword: string) {
    return apiFetch<null>(`/users/${requireUserId()}/password`, {
      method: 'POST',
      body: { current_password: currentPassword, new_password: newPassword },
    })
  }

  /** Triggers the bulk import of all public decks of the linked moxfield_username. */
  function startMoxfieldImport() {
    return apiFetch<MoxfieldImportJob>('/moxfield-import', { method: 'POST' })
  }

  function getMoxfieldImportStatus(jobId: string) {
    return apiFetch<MoxfieldImportJob>(`/moxfield-import/${jobId}`)
  }

  /**
   * The most recently started import job for the current user, whatever its
   * status, or null if they never ran one. Used on mount to resume tracking
   * a job across page navigations/reloads (settings.vue only keeps the job
   * ID in memory otherwise, so leaving the page loses it even though the
   * import itself keeps running in the background).
   */
  async function getLatestMoxfieldImportStatus() {
    try {
      return await apiFetch<MoxfieldImportJob>('/moxfield-import')
    } catch (err) {
      if (apiErrorStatus(err) === 404) return null
      throw err
    }
  }

  return {
    updateMoxfieldUsername,
    updateUsername,
    changePassword,
    startMoxfieldImport,
    getMoxfieldImportStatus,
    getLatestMoxfieldImportStatus,
  }
}

/**
 * Translates POST /users/{id}/password errors. 401 covers two distinct
 * domain cases (incorrect current password and a Google account with no
 * password of its own, see internal/users/service.go): they're distinguished by the
 * message content, same approach as moxfieldImportError with the 400
 * for "no commander".
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

/** Translates PATCH /users/{id} errors when updating moxfield_username. */
export function updateMoxfieldUsernameError(err: unknown): string {
  return apiErrorMessage(err, useI18n().t('errors.updateMoxfieldUsername.generic'))
}

/** Translates PATCH /users/{id} errors when updating the login username. */
export function updateUsernameError(err: unknown): string {
  const { t } = useI18n()
  if (apiErrorStatus(err) === 409) return t('errors.updateUsername.taken')
  return apiErrorMessage(err, t('errors.updateUsername.generic'))
}

/**
 * Translates POST /moxfield-import errors. Listing decks on Moxfield and
 * importing them both happen in the background (internal/moxfieldimport):
 * this call itself can only fail synchronously on 400/409 (a Moxfield
 * failure surfaces later, on the job's own 'failed' status/error_message,
 * see importJob.error_message in settings.vue).
 */
export function startMoxfieldImportError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.startMoxfieldImport.needUsername')
    case 409:
      return t('errors.startMoxfieldImport.inProgress')
    default:
      return apiErrorMessage(err, t('errors.startMoxfieldImport.generic'))
  }
}
