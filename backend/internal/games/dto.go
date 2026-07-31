package games

// CreateGameRequest is the payload for creating a new game.
type CreateGameRequest struct {
	PlaygroupID string `json:"playgroup_id,omitempty"`
}

// GameResponse is the DTO of a game sent to the client.
type GameResponse struct {
	ID                  string               `json:"id"`
	PlaygroupID         *string              `json:"playgroup_id,omitempty"`
	Status              string               `json:"status"`
	StartedAt           *string              `json:"started_at,omitempty"`
	FinishedAt          *string              `json:"finished_at,omitempty"`
	CurrentTurnPlayerID *string              `json:"current_turn_player_id,omitempty"`
	Players             []GamePlayerResponse `json:"players,omitempty"`
}

// GameListResponse is a page of the game history. NextCursor is the
// cursor to pass as the `cursor` query param to request the next page, or null
// if this was the last one. The cursor is opaque: the client returns it as-is.
type GameListResponse struct {
	Items      []GameResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

// JoinGameRequest is the payload for joining a game. Without user_id, the
// player is the authenticated user (via JWT), as usual. With user_id
// (proxy-join, see ADR-0013), the caller adds ANOTHER user on their behalf —
// only allowed if the game belongs to a playgroup and both share it;
// in that case deck_id must belong to the given user, not
// the caller.
type JoinGameRequest struct {
	DeckID string `json:"deck_id"`
	UserID string `json:"user_id,omitempty"`
}

// GamePlayerResponse is the DTO of a player's state within a game.
type GamePlayerResponse struct {
	ID                 string `json:"id"`
	GameID             string `json:"game_id"`
	UserID             string `json:"user_id"`
	DeckID             string `json:"deck_id"`
	LifeTotal          int32  `json:"life_total"`
	PoisonCounters     int32  `json:"poison_counters"`
	EnergyCounters     int32  `json:"energy_counters"`
	ExperienceCounters int32  `json:"experience_counters"`
	IsEliminated       bool   `json:"is_eliminated"`
}
