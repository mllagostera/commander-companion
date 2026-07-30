// Tipos de la API Go. Espejo de los schemas de docs/api/openapi.yaml.

/**
 * Envoltorio de paginación cursor-based de `GET /decks` y `GET /games`
 * (`DeckListResponse`/`GameListResponse`). `next_cursor` es `null` en la última página.
 * Ningún consumidor de este cliente pagina todavía — se lee siempre `items` entero.
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
 * Respuesta de `POST /sync/moxfield` y `GET /sync/status`.
 * status: "updated"/"unchanged" en el POST (según Moxfield trajo cambios o
 * no), "synced"/"never_synced" en el GET (según el deck tenga o no un sync
 * previo registrado).
 */
export interface SyncResponse {
  status: 'updated' | 'unchanged' | 'synced' | 'never_synced'
  deck: Deck
  last_synced_at: string | null
}

/**
 * Job de importación masiva de Moxfield (`POST /moxfield-import`,
 * `GET /moxfield-import/{jobId}`). Importa en background todos los decks
 * públicos del moxfield_username vinculado al perfil.
 */
export interface MoxfieldImportJob {
  id: string
  moxfield_username: string
  status: 'in_progress' | 'completed' | 'failed'
  total_decks: number | null
  imported_count: number
  failed_count: number
  error_message: string | null
}

/**
 * Job de resync masivo de decks (`POST /decks/resync-all`,
 * `GET /decks/resync-all/{jobId}`). Actualiza en background todos los decks
 * YA IMPORTADOS que tengan moxfield_id — distinto de MoxfieldImportJob, que
 * trae decks nuevos a partir de un username.
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
 * Grupo de juego (`GET/POST /playgroups`, `GET /playgroups/{id}`). `members`
 * viene poblado tanto en el listado como en el detalle.
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
 * Resultado de `GET /users/search`. A diferencia de un miembro de playgroup, no trae
 * `playgroup_id` (no está atado a ningún grupo todavía) ni email (ver UserSearchResult
 * en el backend — nunca se expone el email de un tercero en una búsqueda).
 */
export interface UserSearchResult {
  id: string
  username: string
}

/** Estado de un jugador dentro de una partida (`GameResponse.players`). */
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
 * Partida (`GET /games`, `GET /games/{id}`, historial de un grupo vía
 * `GET /games?playgroup_id=`). `players` viene poblado en el detalle y en el
 * historial de grupo, pero no en el listado global paginado.
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
