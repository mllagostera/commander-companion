package statistics

// UserStatsResponse is the DTO for a user's global statistics.
type UserStatsResponse struct {
	UserID               string `json:"user_id"`
	GamesPlayed          int32  `json:"games_played"`
	GamesWon             int32  `json:"games_won"`
	TotalDamageDealt     int32  `json:"total_damage_dealt"`
	TotalCommanderDamage int32  `json:"total_commander_damage_dealt"`
	TotalEliminations    int32  `json:"total_eliminations"`
}

// DeckStatsResponse is the DTO for a deck's statistics.
type DeckStatsResponse struct {
	DeckID               string `json:"deck_id"`
	GamesPlayed          int32  `json:"games_played"`
	GamesWon             int32  `json:"games_won"`
	HighestLifeTotal     int32  `json:"highest_life_total_achieved"`
	TotalCommanderDamage int32  `json:"total_commander_damage_dealt"`
}

// PlaygroupStatsResponse is the DTO for a playgroup's aggregated statistics,
// calculated live (there's no precalculated per-playgroup summary table).
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
