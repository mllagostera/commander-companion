package decks

import "time"

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

// DeckListResponse es una página de decks. NextCursor es el cursor a pasar como
// query param `cursor` para pedir la página siguiente, o null si esta era la
// última. El cursor es opaco: el cliente lo devuelve tal cual.
type DeckListResponse struct {
	Items      []DeckResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

// MoxfieldSyncState describe el estado de sincronización de un deck importado de
// Moxfield: cómo está guardado ahora, cuándo fue su último sync (nil si se importó
// y nunca se re-sincronizó) y si el último sync trajo cambios —Changed siempre es
// false cuando solo se consultó el estado sin llamar a Moxfield—.
type MoxfieldSyncState struct {
	Deck         *DeckResponse
	LastSyncedAt *time.Time
	Changed      bool
}
