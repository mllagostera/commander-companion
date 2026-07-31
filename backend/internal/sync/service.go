// Package sync re-syncs a deck that's ALREADY imported with its current version on
// Moxfield. It's the counterpart of POST /decks/import/moxfield, which is a
// one-off import: here the deck already exists and only its content gets refreshed.
//
// Scope decision: the sync is SYNCHRONOUS. The original stub promised a
// background job model ("queued"/"in_progress" + job_id), but a real queue
// would mean pulling in a new hard dependency (broker + worker + jobs table)
// that the rest of the project doesn't have, and the actual work is a single
// HTTP call to Moxfield plus an UPDATE: on the order of hundreds of milliseconds,
// tolerable within the request. As a consequence:
//
//   - POST /sync/moxfield responds 200 with the result already applied (previously: 202
//     with a made-up job_id).
//   - There's no job_id: GET /sync/status is identified by moxfield_id and reports the
//     deck's stored state (when its last sync happened), not a job's progress.
//
// If at some point sync becomes expensive (several decks at once, Moxfield rate
// limits), this is the point where the queue should be introduced, without touching
// clients beyond the status code.
package sync

import (
	"context"

	"github.com/usuario/commander-companion-backend/internal/decks"
)

const (
	// statusUpdated indicates that Moxfield had changes and the deck was updated.
	statusUpdated = "updated"
	// statusUnchanged indicates that the deck already matched what's on Moxfield.
	statusUnchanged = "unchanged"
	// statusSynced indicates that the deck has at least one successful sync recorded.
	statusSynced = "synced"
	// statusNeverSynced indicates that the deck was imported and never re-synced.
	statusNeverSynced = "never_synced"
)

// DeckResyncer is what sync needs from the decks module: the deck's state and
// writes live there (it owns the table and the Moxfield client), sync is just
// the transport for the operation. As an interface, it allows mocking it in tests
// (same pattern as games.StatisticsRecalculator).
type DeckResyncer interface {
	ResyncFromMoxfield(ctx context.Context, userID, moxfieldID string) (*decks.MoxfieldSyncState, error)
	GetMoxfieldSyncState(ctx context.Context, userID, moxfieldID string) (*decks.MoxfieldSyncState, error)
}

// Service defines the business logic of the sync module.
type Service interface {
	TriggerSync(ctx context.Context, userID, moxfieldID string) (*Response, error)
	GetSyncStatus(ctx context.Context, userID, moxfieldID string) (*Response, error)
}

type service struct {
	decks DeckResyncer
}

// NewService creates a new sync service.
func NewService(deckResyncer DeckResyncer) Service {
	return &service{decks: deckResyncer}
}

// TriggerSync queries Moxfield again for the user's deck associated with moxfieldID
// and applies the changes. Returns decks.ErrDeckNotFound (→ 404) if the user hasn't
// imported any deck with that ID.
func (s *service) TriggerSync(ctx context.Context, userID, moxfieldID string) (*Response, error) {
	state, err := s.decks.ResyncFromMoxfield(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}

	status := statusUnchanged
	if state.Changed {
		status = statusUpdated
	}
	return toResponse(status, state), nil
}

// GetSyncStatus returns the deck's stored sync state, without calling
// Moxfield.
func (s *service) GetSyncStatus(ctx context.Context, userID, moxfieldID string) (*Response, error) {
	state, err := s.decks.GetMoxfieldSyncState(ctx, userID, moxfieldID)
	if err != nil {
		return nil, err
	}

	status := statusSynced
	if state.LastSyncedAt == nil {
		status = statusNeverSynced
	}
	return toResponse(status, state), nil
}
