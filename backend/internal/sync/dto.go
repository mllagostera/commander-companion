package sync

import (
	"time"

	"github.com/usuario/commander-companion-backend/internal/decks"
)

// Request es el payload para sincronizar un deck ya importado con Moxfield.
// MoxfieldID acepta tanto el ID público como la URL completa del deck, igual que
// el import (ver moxfield.ExtractPublicID).
type Request struct {
	MoxfieldID string `json:"moxfield_id"`
}

// Response es el resultado de una sincronización con Moxfield.
//
// Status vale "updated" o "unchanged" en POST /sync/moxfield, según Moxfield haya
// traído cambios o no; y "synced" o "never_synced" en GET /sync/status, según el
// deck tenga o no un sync previo registrado.
//
// LastSyncedAt es null solo en el segundo caso: un deck importado que nunca se
// re-sincronizó.
type Response struct {
	Status       string             `json:"status"`
	Deck         decks.DeckResponse `json:"deck"`
	LastSyncedAt *string            `json:"last_synced_at"`
}

// toResponse arma el DTO HTTP a partir del estado que devuelve el módulo decks.
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
