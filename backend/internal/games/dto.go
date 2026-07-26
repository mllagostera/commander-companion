package games

// CreateGameRequest es el payload para crear una nueva partida.
type CreateGameRequest struct {
	PlaygroupID string `json:"playgroup_id,omitempty"`
}

// GameResponse es el DTO de una partida enviado al cliente.
type GameResponse struct {
	ID          string               `json:"id"`
	PlaygroupID *string              `json:"playgroup_id,omitempty"`
	Status      string               `json:"status"`
	StartedAt   *string              `json:"started_at,omitempty"`
	FinishedAt  *string              `json:"finished_at,omitempty"`
	Players     []GamePlayerResponse `json:"players,omitempty"`
}

// JoinGameRequest es el payload para unirse a una partida. El jugador es
// siempre el usuario autenticado (vía JWT); solo hace falta indicar el deck.
type JoinGameRequest struct {
	DeckID string `json:"deck_id"`
}

// GamePlayerResponse es el DTO del estado de un jugador dentro de una partida.
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
