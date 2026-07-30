package deckresync

// JobResponse es el DTO de un job de resync masivo enviado al cliente.
type JobResponse struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	TotalDecks   int32   `json:"total_decks"`
	UpdatedCount int32   `json:"updated_count"`
	FailedCount  int32   `json:"failed_count"`
	ErrorMessage *string `json:"error_message,omitempty"`
}
