package gameactions

// CreateActionRequest is the payload for recording a game action.
type CreateActionRequest struct {
	ActorID    string                 `json:"actor_id"`
	TargetID   string                 `json:"target_id,omitempty"`
	ActionType string                 `json:"action_type"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// GameActionResponse is the DTO of a game action sent to the client.
type GameActionResponse struct {
	ID         string                 `json:"id"`
	GameID     string                 `json:"game_id"`
	ActorID    string                 `json:"actor_id"`
	TargetID   *string                `json:"target_id,omitempty"`
	ActionType string                 `json:"action_type"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	CreatedAt  string                 `json:"created_at"`
}
