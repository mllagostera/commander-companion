package games

// CreateGameRequest es el payload para crear una nueva partida.
type CreateGameRequest struct {
	PlaygroupID string `json:"playgroup_id,omitempty"`
}

// GameResponse es el DTO de una partida enviado al cliente.
type GameResponse struct {
	ID                  string               `json:"id"`
	PlaygroupID         *string              `json:"playgroup_id,omitempty"`
	Status              string               `json:"status"`
	StartedAt           *string              `json:"started_at,omitempty"`
	FinishedAt          *string              `json:"finished_at,omitempty"`
	CurrentTurnPlayerID *string              `json:"current_turn_player_id,omitempty"`
	Players             []GamePlayerResponse `json:"players,omitempty"`
}

// GameListResponse es una página del historial de partidas. NextCursor es el
// cursor a pasar como query param `cursor` para pedir la página siguiente, o null
// si esta era la última. El cursor es opaco: el cliente lo devuelve tal cual.
type GameListResponse struct {
	Items      []GameResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
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
