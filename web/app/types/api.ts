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
