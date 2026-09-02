import type {
  PaginatedResponse,
  Tournament,
  TournamentDetail,
  TournamentLookup,
  TournamentParticipant,
  TournamentTable,
} from '~/types/api'

export interface SeatResultInput {
  participant_id: string
  finish_position: number
}

export function useTournaments() {
  const { apiFetch } = useApi()

  /** One page of tournaments the caller organizes or participates in. */
  function listTournamentsPage(cursor?: string): Promise<PaginatedResponse<Tournament>> {
    return apiFetch<PaginatedResponse<Tournament>>('/tournaments', { query: cursor ? { cursor } : undefined })
  }

  /** The caller becomes its organizer. targetPlayers is display-only, not enforced. */
  function createTournament(name: string, targetPlayers?: number) {
    return apiFetch<Tournament>('/tournaments', {
      method: 'POST',
      body: { name: name.trim(), target_players: targetPlayers },
    })
  }

  /** Standings and, once left registration, every round played so far. */
  function getTournament(id: string) {
    return apiFetch<TournamentDetail>(`/tournaments/${id}`)
  }

  /** Organizer-only, and only while status is 'registration': undoes a tournament created by mistake. */
  function deleteTournament(id: string) {
    return apiFetch<null>(`/tournaments/${id}`, { method: 'DELETE' })
  }

  /** Self-registers the caller with one of their own decks. Only while status is 'registration'. */
  function joinTournament(joinCode: string, deckId: string) {
    return apiFetch<TournamentParticipant>('/tournaments/join', {
      method: 'POST',
      body: { join_code: joinCode.trim(), deck_id: deckId },
    })
  }

  /** Organizer-only: registers a participant with no account. */
  function addGuestParticipant(tournamentId: string, guestName: string, commanderName: string) {
    return apiFetch<TournamentParticipant>(`/tournaments/${tournamentId}/participants`, {
      method: 'POST',
      body: { guest_name: guestName.trim(), commander_name: commanderName.trim() },
    })
  }

  /** Organizer-only: locks the roster and seats round 1. */
  function startTournament(id: string) {
    return apiFetch<TournamentDetail>(`/tournaments/${id}/start`, { method: 'POST' })
  }

  /** Organizer-only: records a table's finish order for the current round. */
  function recordTableResult(tournamentId: string, tableId: string, results: SeatResultInput[]) {
    return apiFetch<TournamentTable>(`/tournaments/${tournamentId}/tables/${tableId}/result`, {
      method: 'POST',
      body: { results },
    })
  }

  /** Organizer-only: finishes the current round and seats the next one, or finishes the tournament. */
  function advanceRound(id: string) {
    return apiFetch<TournamentDetail>(`/tournaments/${id}/rounds/next`, { method: 'POST' })
  }

  /** The "enter the code" lookup: public summary, plus the caller's own status if they're a participant. */
  function lookupByCode(code: string) {
    return apiFetch<TournamentLookup>('/tournaments/lookup', { query: { code: code.trim() } })
  }

  return {
    listTournamentsPage,
    createTournament,
    getTournament,
    deleteTournament,
    joinTournament,
    addGuestParticipant,
    startTournament,
    recordTableResult,
    advanceRound,
    lookupByCode,
  }
}

export function createTournamentError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.tournaments.create.needName')
    default:
      return apiErrorMessage(err, t('errors.tournaments.create.generic'))
  }
}

/** See ErrTournamentNotFound (404, invalid code), ErrAlreadyJoined/ErrTournamentNotOpen (409) in internal/tournaments/service.go. */
export function joinTournamentError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.tournaments.join.invalidCode')
    case 409:
      return t('errors.tournaments.join.alreadyJoinedOrClosed')
    default:
      return apiErrorMessage(err, t('errors.tournaments.join.generic'))
  }
}

export function addGuestParticipantError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.tournaments.addGuest.missingFields')
    case 404:
      return t('errors.tournaments.addGuest.notOrganizer')
    case 409:
      return t('errors.tournaments.addGuest.closed')
    default:
      return apiErrorMessage(err, t('errors.tournaments.addGuest.generic'))
  }
}

/** See ErrTournamentNotFound (404, not yours) and ErrTournamentNotDeletable (409) in internal/tournaments/service.go. */
export function deleteTournamentError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.tournaments.delete.notOrganizer')
    case 409:
      return t('errors.tournaments.delete.alreadyStarted')
    default:
      return apiErrorMessage(err, t('errors.tournaments.delete.generic'))
  }
}

export function startTournamentError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.tournaments.start.invalidCount')
    case 404:
      return t('errors.tournaments.start.notOrganizer')
    case 409:
      return t('errors.tournaments.start.alreadyStarted')
    default:
      return apiErrorMessage(err, t('errors.tournaments.start.generic'))
  }
}

export function recordTableResultError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 400:
      return t('errors.tournaments.recordResult.invalid')
    case 409:
      return t('errors.tournaments.recordResult.notInProgress')
    default:
      return apiErrorMessage(err, t('errors.tournaments.recordResult.generic'))
  }
}

export function advanceRoundError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 409:
      return t('errors.tournaments.advanceRound.notComplete')
    default:
      return apiErrorMessage(err, t('errors.tournaments.advanceRound.generic'))
  }
}

export function lookupTournamentError(err: unknown): string {
  const { t } = useI18n()
  switch (apiErrorStatus(err)) {
    case 404:
      return t('errors.tournaments.lookup.notFound')
    default:
      return apiErrorMessage(err, t('errors.tournaments.lookup.generic'))
  }
}
