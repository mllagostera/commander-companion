package sync

import (
	"time"

	"github.com/usuario/commander-companion-backend/internal/decks"
)

// Request is the payload for syncing a deck already imported with Moxfield.
// MoxfieldID accepts both the public ID and the deck's full URL, same as
// the import (see moxfield.ExtractPublicID).
type Request struct {
	MoxfieldID string `json:"moxfield_id"`
}

// Response is the result of a sync with Moxfield.
//
// Status is "updated" or "unchanged" in POST /sync/moxfield, depending on whether
// Moxfield brought changes or not; and "synced" or "never_synced" in GET /sync/status,
// depending on whether the deck has a previous sync recorded or not.
//
// LastSyncedAt is null only in the second case: a deck that was imported but never
// re-synced.
type Response struct {
	Status       string             `json:"status"`
	Deck         decks.DeckResponse `json:"deck"`
	LastSyncedAt *string            `json:"last_synced_at"`
}

// toResponse builds the HTTP DTO from the state returned by the decks module.
func toResponse(status string, state *decks.MoxfieldSyncState) *Response {
	res := &Response{Status: status}
	if state.Deck != nil {
		res.Deck = *state.Deck
	}
	if state.LastSyncedAt != nil {
		formatted := state.LastSyncedAt.Format(time.RFC3339)
		res.LastSyncedAt = &formatted
	}
	return res
}
