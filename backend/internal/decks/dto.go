package decks

// CreateDeckRequest es el payload para crear un deck manualmente.
type CreateDeckRequest struct {
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
}

// DeckResponse es el DTO de un deck enviado al cliente.
type DeckResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
}
