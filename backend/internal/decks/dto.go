package decks

import (
	"encoding/json"
	"time"
)

// CreateDeckRequest is the payload for creating a deck manually.
type CreateDeckRequest struct {
	Name       string `json:"name"`
	Commander  string `json:"commander"`
	MoxfieldID string `json:"moxfield_id,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	// Bracket and ColorIdentity are optional; a value given here counts as
	// set by hand (see UpdateDeckRequest).
	Bracket       *int     `json:"bracket,omitempty"`
	ColorIdentity []string `json:"color_identity,omitempty"`
}

// UpdateDeckRequest is the payload for PATCH /decks/{id}. Bracket and
// ColorIdentity are kept raw so a field that's absent (leave it alone) can be
// told apart from one sent as null (clear it to unknown).
type UpdateDeckRequest struct {
	Bracket            json.RawMessage `json:"bracket"`
	ColorIdentity      json.RawMessage `json:"color_identity"`
	ResetBracket       bool            `json:"reset_bracket"`
	ResetColorIdentity bool            `json:"reset_color_identity"`
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
	// Bracket and ColorIdentity are always present: null is unknown, and an
	// empty ColorIdentity is colorless.
	Bracket                 *int     `json:"bracket"`
	ColorIdentity           []string `json:"color_identity"`
	BracketOverridden       bool     `json:"bracket_overridden"`
	ColorIdentityOverridden bool     `json:"color_identity_overridden"`
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
