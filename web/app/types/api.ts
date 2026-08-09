// Go API types. Mirrors the schemas from docs/api/openapi.yaml.

/**
 * Cursor-based pagination wrapper for `GET /decks` and `GET /games`
 * (`DeckListResponse`/`GameListResponse`). `next_cursor` is `null` on the last page.
 * No consumer of this client paginates yet — `items` is always read in full.
 */
export interface PaginatedResponse<T> {
  items: T[]
  next_cursor: string | null
}

export interface Deck {
  id: string
  user_id: string
  name: string
  commander: string
  moxfield_id: string | null
  image_url: string | null
}

/**
 * Response from `POST /sync/moxfield` and `GET /sync/status`.
 * status: "updated"/"unchanged" on the POST (depending on whether Moxfield had
 * changes or not), "synced"/"never_synced" on the GET (depending on whether the
 * deck has a previous sync recorded or not).
 */
export interface SyncResponse {
  status: 'updated' | 'unchanged' | 'synced' | 'never_synced'
  deck: Deck
  last_synced_at: string | null
}

/**
 * Bulk Moxfield import job (`POST /moxfield-import`,
 * `GET /moxfield-import/{jobId}`). Imports in the background all the public
 * decks of the moxfield_username linked to the profile.
 */
export interface MoxfieldImportJob {
  id: string
  moxfield_username: string
  // 'pending': the job was created but the deck list hasn't been fetched from
  // Moxfield yet (total_decks is still null at that point).
  status: 'pending' | 'in_progress' | 'completed' | 'failed'
  total_decks: number | null
  imported_count: number
  failed_count: number
  error_message: string | null
}

/**
 * Bulk deck resync job (`POST /decks/resync-all`,
 * `GET /decks/resync-all/{jobId}`). Updates in the background all decks
 * ALREADY IMPORTED that have a moxfield_id — different from MoxfieldImportJob, which
 * brings in new decks from a username.
 */
export interface DeckResyncJob {
  id: string
  status: 'in_progress' | 'completed' | 'failed'
  total_decks: number
  updated_count: number
  failed_count: number
  error_message: string | null
}

export interface UserStats {
  user_id: string
  games_played: number
  games_won: number
  total_damage_dealt: number
  total_commander_damage_dealt: number
  total_eliminations: number
}

export interface DeckStats {
  deck_id: string
  games_played: number
  games_won: number
  highest_life_total_achieved: number
  total_commander_damage_dealt: number
}

/**
 * Playgroup (`GET/POST /playgroups`, `GET /playgroups/{id}`). `members`
 * comes populated both in the listing and in the detail view.
 */
export interface Playgroup {
  id: string
  name: string
  members?: PlaygroupMember[]
}

export interface PlaygroupMember {
  playgroup_id: string
  user_id: string
  username: string
}

/**
 * Result of `GET /users/search`. Unlike a playgroup member, it doesn't carry
 * `playgroup_id` (not tied to any group yet) or email (see UserSearchResult
 * in the backend — a third party's email is never exposed in a search).
 */
export interface UserSearchResult {
  id: string
  username: string
}

/** State of a player within a game (`GameResponse.players`). */
export interface GamePlayer {
  id: string
  game_id: string
  user_id: string
  deck_id: string
  life_total: number
  poison_counters: number
  energy_counters: number
  experience_counters: number
  is_eliminated: boolean
}

/**
 * Game (`GET /games`, `GET /games/{id}`, a group's history via
 * `GET /games?playgroup_id=`). `players` comes populated in the detail view and in the
 * group history, but not in the paginated global listing.
 */
export interface Game {
  id: string
  playgroup_id?: string
  status: 'pending' | 'active' | 'finished'
  started_at: string | null
  finished_at: string | null
  current_turn_player_id?: string
  players?: GamePlayer[]
}

export interface PlaygroupMemberStats {
  user_id: string
  games_played: number
  games_won: number
}

export interface PlaygroupStats {
  playgroup_id: string
  games_played: number
  members: PlaygroupMemberStats[]
}

/** One entry of `GET /statistics/playgroups` -- replaces one PlaygroupStats call per group. */
export interface PlaygroupGameCount {
  playgroup_id: string
  playgroup_name: string
  games_played: number
}

/** One entry of `GET /statistics/opponents` -- the head-to-head record against one opponent. */
export interface OpponentStats {
  user_id: string
  username: string
  games_together: number
  times_you_eliminated_them: number
  times_eliminated_by_opponent: number
}

/** One seat within a FinishedGame, already enriched server-side (no client-side lookup needed). */
export interface FinishedGamePlayer {
  user_id: string
  username: string
  deck_id: string
  deck_name: string
  deck_commander: string
  deck_image_url: string | null
  won: boolean
}

/** The single largest CombatDamage/CommanderDamage hit dealt within a FinishedGame. */
export interface BiggestHit {
  amount: number
  username: string
}

/** One item of `GET /statistics/games` (paginated via PaginatedResponse). */
export interface FinishedGame {
  id: string
  playgroup_id: string | null
  playgroup_name: string | null
  started_at: string | null
  finished_at: string | null
  players: FinishedGamePlayer[]
  /** Total turns played (every TurnStart action belongs to one player's turn). */
  turn_count: number
  /** Null if the game has no CombatDamage/CommanderDamage actions logged. */
  biggest_hit: BiggestHit | null
}

/**
 * Standalone Swiss-format Commander tournament (`GET/POST /tournaments`,
 * `GET /tournaments/{id}`). Not tied to a playgroup — any authenticated user
 * can create one. `join_code` is what participants use to self-register
 * (`POST /tournaments/join`) and later look up their table (`GET /tournaments/lookup`).
 */
export interface Tournament {
  id: string
  organizer_id: string
  name: string
  format: 'commander'
  /** Display-only planning target set at creation, not enforced. */
  target_players: number | null
  status: 'registration' | 'in_progress' | 'finished'
  /** Computed and locked in once the tournament starts. */
  round_count: number | null
  current_round: number
  join_code: string
  created_at: string
  started_at: string | null
  finished_at: string | null
}

/** A tournament participant: exactly one of user_id/guest_name is set. */
export interface TournamentParticipant {
  id: string
  user_id: string | null
  username: string | null
  guest_name: string | null
  deck_id: string | null
  deck_name: string | null
  /** The linked deck's commander if deck_id is set, otherwise the guest's free-text commander. */
  commander_name: string
  points: number
}

/** One participant's seat at a table, with their result once the organizer records it. */
export interface TournamentSeat {
  id: string
  participant_id: string
  user_id: string | null
  username: string | null
  guest_name: string | null
  commander_name: string
  /** Null until POST /tournaments/{id}/tables/{tableId}/result is called for this table. */
  finish_position: number | null
  points_awarded: number
}

/** One table (pod, 3-4 seats) within a tournament round. */
export interface TournamentTable {
  id: string
  table_number: number
  seats: TournamentSeat[]
}

/** One round of a tournament: its tables and each seat's result. */
export interface TournamentRound {
  round_number: number
  status: 'pending' | 'finished'
  tables: TournamentTable[]
}

/** Full detail of a tournament (`GET /tournaments/{id}`). */
export interface TournamentDetail {
  tournament: Tournament
  /** Standings order: most points first. */
  participants: TournamentParticipant[]
  /** Every round played so far. Absent while status is 'registration'. */
  rounds?: TournamentRound[]
}

/**
 * Response of `GET /tournaments/lookup?code=` — the "enter the code" screen:
 * the tournament's public summary, plus the caller's own status if they're a participant.
 */
export interface TournamentLookup {
  tournament: Tournament
  /** The caller's own participant record. Absent if they haven't joined. */
  participant?: TournamentParticipant
  /**
   * The caller's table assignment for the tournament's current round. Only
   * set once status is 'in_progress' and they have a seat in it.
   */
  current_table?: TournamentTable
}
