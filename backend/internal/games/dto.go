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

// JoinGameRequest es el payload para unirse a una partida. Sin user_id, el
// jugador es el usuario autenticado (vía JWT), como siempre. Con user_id
// (proxy-join, ver ADR-0013), el caller une a OTRO usuario en su nombre —
// solo permitido si la partida pertenece a un playgroup y ambos lo
// comparten; en ese caso deck_id debe pertenecer al usuario indicado, no
// al caller.
type JoinGameRequest struct {
	DeckID string `json:"deck_id"`
	UserID string `json:"user_id,omitempty"`
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
