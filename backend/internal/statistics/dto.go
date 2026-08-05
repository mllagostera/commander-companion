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

// PlaygroupMemberStats summarizes the games played/won by a member within the group.
type PlaygroupMemberStats struct {
	UserID      string `json:"user_id"`
	GamesPlayed int32  `json:"games_played"`
	GamesWon    int32  `json:"games_won"`
}

// OpponentStatsResponse is the DTO for the head-to-head record against one
// other user, across every finished game they've shared a seat in.
type OpponentStatsResponse struct {
	UserID                    string `json:"user_id"`
	Username                  string `json:"username"`
	GamesTogether             int32  `json:"games_together"`
	TimesYouEliminatedThem    int32  `json:"times_you_eliminated_them"`
	TimesEliminatedByOpponent int32  `json:"times_eliminated_by_opponent"`
}

// PlaygroupGameCountResponse is the DTO for how many finished games the
// authenticated user has played within one of their playgroups.
type PlaygroupGameCountResponse struct {
	PlaygroupID   string `json:"playgroup_id"`
	PlaygroupName string `json:"playgroup_name"`
	GamesPlayed   int32  `json:"games_played"`
}

// FinishedGameResponse is one entry of the finished-games history
// (GET /statistics/games): a finished game enriched with every seat's
// username/deck, so the client doesn't need a lookup per player.
type FinishedGameResponse struct {
	ID            string                       `json:"id"`
	PlaygroupID   *string                      `json:"playgroup_id,omitempty"`
	PlaygroupName *string                      `json:"playgroup_name,omitempty"`
	StartedAt     *string                      `json:"started_at,omitempty"`
	FinishedAt    *string                      `json:"finished_at,omitempty"`
	Players       []FinishedGamePlayerResponse `json:"players"`
	// TurnCount is how many turns were played in total (every TurnStart action
	// belongs to one player's turn, see ListGameActionSummaryForGames). Zero for a
	// game whose actions weren't tracked turn-by-turn.
	TurnCount int32 `json:"turn_count"`
	// BiggestHit is the single largest CombatDamage/CommanderDamage hit dealt in the
	// game, if any were recorded.
	BiggestHit *BiggestHitResponse `json:"biggest_hit,omitempty"`
}

// BiggestHitResponse is the single largest hit dealt within a FinishedGameResponse.
type BiggestHitResponse struct {
	Amount   int32  `json:"amount"`
	Username string `json:"username"`
}

// FinishedGamePlayerResponse is one seat within a FinishedGameResponse.
type FinishedGamePlayerResponse struct {
	UserID        string  `json:"user_id"`
	Username      string  `json:"username"`
	DeckID        string  `json:"deck_id"`
	DeckName      string  `json:"deck_name"`
	DeckCommander string  `json:"deck_commander"`
	DeckImageURL  *string `json:"deck_image_url,omitempty"`
	Won           bool    `json:"won"`
}

// FinishedGameListResponse is a page of the finished-games history.
// NextCursor is the cursor to pass as the `cursor` query param to request the
// next page, or null if this was the last one. The cursor is opaque: the
// client returns it as-is.
type FinishedGameListResponse struct {
	Items      []FinishedGameResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}
