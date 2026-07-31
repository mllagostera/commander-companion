package decks

import "time"

// CreateDeckRequest is the payload for creating a deck manually.
type CreateDeckRequest struct {
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
}

// ImportMoxfieldRequest is the payload for importing a deck from Moxfield.
type ImportMoxfieldRequest struct {
	// URL accepts both the full URL (https://moxfield.com/decks/{id})
	// and just the deck's public ID.
	URL string `json:"url"`
}

// DeckResponse is the DTO for a deck sent to the client.
type DeckResponse struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
}

// DeckListResponse is a page of decks. NextCursor is the cursor to pass as the
// `cursor` query param to request the next page, or null if this was the
// last one. The cursor is opaque: the client returns it as-is.
type DeckListResponse struct {
	Items      []DeckResponse `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

// MoxfieldSyncState describes the sync state of a deck imported from
// Moxfield: how it's currently stored, when its last sync happened (nil if it was
// imported and never re-synced), and whether the last sync brought changes —Changed is
// always false when only the state was queried without calling Moxfield—.
type MoxfieldSyncState struct {
	Deck         *DeckResponse
	LastSyncedAt *time.Time
	Changed      bool
}
