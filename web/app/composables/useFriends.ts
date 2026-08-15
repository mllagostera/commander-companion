import type { Friend, FriendRequestResult, IncomingFriendRequest, OutgoingFriendRequest } from '~/types/api'

export function useFriends() {
  const { apiFetch } = useApi()

  /**
   * userId is resolved client-side beforehand, either via useUsers().searchUsers
   * (typing a username) or by decoding another user's profile QR (future work,
   * see ADR-0017) -- both entry points call this same endpoint.
   */
  function sendFriendRequest(userId: string) {
    return apiFetch<FriendRequestResult>('/friends/requests', {
      method: 'POST',
      body: { addressee_id: userId },
    })
  }

  function listIncomingRequests() {
    return apiFetch<IncomingFriendRequest[]>('/friends/requests', { query: { direction: 'incoming' } })
  }

  function listOutgoingRequests() {
    return apiFetch<OutgoingFriendRequest[]>('/friends/requests', { query: { direction: 'outgoing' } })
  }

  /** Only the request's addressee may accept it. */
  function acceptFriendRequest(requestId: string) {
    return apiFetch<Friend>(`/friends/requests/${requestId}/accept`, { method: 'POST' })
  }

  /** Only the request's addressee may reject it. */
  function rejectFriendRequest(requestId: string) {
    return apiFetch<null>(`/friends/requests/${requestId}/reject`, { method: 'POST' })
  }

  /** Only the request's original sender may cancel it. */
  function cancelFriendRequest(requestId: string) {
    return apiFetch<null>(`/friends/requests/${requestId}`, { method: 'DELETE' })
  }

  function listFriends() {
    return apiFetch<Friend[]>('/friends')
  }

  function removeFriend(userId: string) {
    return apiFetch<null>(`/friends/${userId}`, { method: 'DELETE' })
  }

  return {
    sendFriendRequest,
    listIncomingRequests,
    listOutgoingRequests,
    acceptFriendRequest,
    rejectFriendRequest,
    cancelFriendRequest,
    listFriends,
    removeFriend,
  }
}

/** See ErrCannotFriendSelf/ErrInvalidUserID (400), ErrUserNotFound (404), ErrAlreadyFriends/ErrRequestAlreadyPending (409) in internal/friends/service.go. */
export function sendFriendRequestError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.friends.send.invalid')
    case 404:
      return t('errors.friends.send.userNotFound')
    case 409:
      return t('errors.friends.send.conflict')
    default:
      return apiErrorMessage(err, t('errors.friends.send.generic'))
  }
}

/** See ErrRequestNotFound (404) and ErrRequestNotPending (409) in internal/friends/service.go. */
export function respondFriendRequestError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.friends.respond.notFound')
    case 409:
      return t('errors.friends.respond.notPending')
    default:
      return apiErrorMessage(err, t('errors.friends.respond.generic'))
  }
}

export function listFriendsError(err: unknown): string {
  const { t } = useI18n()
  return apiErrorMessage(err, t('errors.friends.list.generic'))
}
