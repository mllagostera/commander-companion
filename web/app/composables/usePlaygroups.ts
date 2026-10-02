import type { Playgroup, PlaygroupMember } from '~/types/api'

export function usePlaygroups() {
  const { apiFetch } = useApi()

  /** Groups the authenticated user is a member of, with `members` populated. */
  function listPlaygroups() {
    return apiFetch<Playgroup[]>('/playgroups')
  }

  /** The creator is automatically added as the first member (done by the backend). */
  function createPlaygroup(name: string) {
    return apiFetch<Playgroup>('/playgroups', {
      method: 'POST',
      body: { name: name.trim() },
    })
  }

  /** The only path with `members` populated. 404 if it doesn't exist or you're not a member. */
  function getPlaygroup(id: string) {
    return apiFetch<Playgroup>(`/playgroups/${id}`)
  }

  /** Only an existing member can rename the group (same approach as addMember). */
  function updatePlaygroup(id: string, name: string) {
    return apiFetch<Playgroup>(`/playgroups/${id}`, {
      method: 'PATCH',
      body: { name: name.trim() },
    })
  }

  /** Creator-only, and only while the group has no games: undoes a group created by mistake. */
  function deletePlaygroup(id: string) {
    return apiFetch<null>(`/playgroups/${id}`, { method: 'DELETE' })
  }

  /** userId must be the UUID of an already existing user; the inviter must already be a member. */
  function addMember(playgroupId: string, userId: string) {
    return apiFetch<PlaygroupMember>(`/playgroups/${playgroupId}/members`, {
      method: 'POST',
      body: { user_id: userId.trim() },
    })
  }

  return { listPlaygroups, createPlaygroup, getPlaygroup, updatePlaygroup, deletePlaygroup, addMember }
}

export function createPlaygroupError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.playgroups.create.needName')
    default:
      return apiErrorMessage(err, t('errors.playgroups.create.generic'))
  }
}

/**
 * 404 here covers both "the group doesn't exist" and "you're not a member" (the backend
 * doesn't distinguish, see getMemberPlaygroup in internal/playgroups/service.go).
 */
export function getPlaygroupError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.playgroups.get.notFoundOrNotMember')
    default:
      return apiErrorMessage(err, t('errors.playgroups.get.generic'))
  }
}

/** Same error mapping as createPlaygroupError/getPlaygroupError: 400 for no name, 404 if not a member. */
export function updatePlaygroupError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.playgroups.update.needName')
    case 404:
      return t('errors.playgroups.update.notFoundOrNotMember')
    default:
      return apiErrorMessage(err, t('errors.playgroups.update.generic'))
  }
}

/** See ErrInvalidUserID (400), ErrUserNotFound (404) and ErrAlreadyMember (409) in internal/playgroups/service.go. */
export function addMemberError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.playgroups.addMember.invalidUserId')
    case 404:
      return t('errors.playgroups.addMember.userNotFound')
    case 409:
      return t('errors.playgroups.addMember.alreadyMember')
    default:
      return apiErrorMessage(err, t('errors.playgroups.addMember.generic'))
  }
}

/** See ErrNotPlaygroupCreator (403), ErrPlaygroupNotFound (404) and ErrPlaygroupHasGames (409) in internal/playgroups/service.go. */
export function deletePlaygroupError(err: unknown): string {
  const { t } = useNuxtApp().$i18n
  switch (apiErrorStatus(err)) {
    case 403:
      return t('errors.playgroups.delete.notCreator')
    case 404:
      return t('errors.playgroups.delete.notFoundOrNotMember')
    case 409:
      return t('errors.playgroups.delete.hasGames')
    default:
      return apiErrorMessage(err, t('errors.playgroups.delete.generic'))
  }
}
