package gameactions

// CreateActionRequest es el payload para registrar una acción de partida.
type CreateActionRequest struct {
	ActorID    string                 `json:"actor_id"`
	TargetID   string                 `json:"target_id,omitempty"`
	ActionType string                 `json:"action_type"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// GameActionResponse es el DTO de una acción de partida enviado al cliente.
type GameActionResponse struct {
	ID         string                 `json:"id"`
	GameID     string                 `json:"game_id"`
	ActorID    string                 `json:"actor_id"`
	TargetID   *string                `json:"target_id,omitempty"`
	ActionType string                 `json:"action_type"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	CreatedAt  string                 `json:"created_at"`
}
