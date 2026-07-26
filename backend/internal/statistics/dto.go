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
	DeckID           string `json:"deck_id"`
	GamesPlayed      int32  `json:"games_played"`
	GamesWon         int32  `json:"games_won"`
	HighestLifeTotal int32  `json:"highest_life_total_achieved"`
}
