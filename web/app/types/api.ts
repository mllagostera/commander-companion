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
