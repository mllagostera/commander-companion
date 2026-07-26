package decks

// CreateDeckRequest es el payload para crear un deck manualmente.
type CreateDeckRequest struct {
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
}

// ImportMoxfieldRequest es el payload para importar un deck desde Moxfield.
type ImportMoxfieldRequest struct {
	// URL acepta tanto la URL completa (https://moxfield.com/decks/{id})
	// como solo el ID público del deck.
	URL string `json:"url"`
}

// DeckResponse es el DTO de un deck enviado al cliente.
type DeckResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
}
