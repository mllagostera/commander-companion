// Tipos de la API Go. Espejo de los schemas de docs/api/openapi.yaml.

export interface Deck {
  id: string
  user_id: string
  name: string
  commander: string
  moxfield_id: string | null
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
