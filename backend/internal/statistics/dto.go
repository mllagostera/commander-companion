package statistics

// UserStatsResponse es el DTO de estadísticas globales de un usuario.
type UserStatsResponse struct {
	UserID               string `json:"user_id"`
	GamesPlayed          int32  `json:"games_played"`
	GamesWon             int32  `json:"games_won"`
	TotalDamageDealt     int32  `json:"total_damage_dealt"`
	TotalCommanderDamage int32  `json:"total_commander_damage_dealt"`
	TotalEliminations    int32  `json:"total_eliminations"`
}

// DeckStatsResponse es el DTO de estadísticas de un deck.
type DeckStatsResponse struct {
	DeckID               string `json:"deck_id"`
	GamesPlayed          int32  `json:"games_played"`
	GamesWon             int32  `json:"games_won"`
	HighestLifeTotal     int32  `json:"highest_life_total_achieved"`
	TotalCommanderDamage int32  `json:"total_commander_damage_dealt"`
}

// PlaygroupStatsResponse es el DTO de estadísticas agregadas de un grupo de juego,
// calculado en vivo (no hay una tabla de resumen precalculada por grupo).
type PlaygroupStatsResponse struct {
	PlaygroupID string                 `json:"playgroup_id"`
	GamesPlayed int32                  `json:"games_played"`
	Members     []PlaygroupMemberStats `json:"members"`
}

// PlaygroupMemberStats resume las partidas jugadas/ganadas de un miembro dentro del grupo.
type PlaygroupMemberStats struct {
	UserID      string `json:"user_id"`
	GamesPlayed int32  `json:"games_played"`
	GamesWon    int32  `json:"games_won"`
}
