package sync_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/sync"
)

// mockResyncer satisface sync.DeckResyncer: el trabajo real (BD + Moxfield) vive en
// internal/decks y ya está cubierto por sus propios tests de integración, así que acá
// solo se verifica la traducción estado-de-deck → DTO de sync, sin tocar Postgres.
type mockResyncer struct {
	state *decks.MoxfieldSyncState
	err   error

	gotUserID     string
	gotMoxfieldID string
}

func (m *mockResyncer) ResyncFromMoxfield(
	_ context.Context, userID, moxfieldID string,
) (*decks.MoxfieldSyncState, error) {
	m.gotUserID, m.gotMoxfieldID = userID, moxfieldID
	return m.state, m.err
}

func (m *mockResyncer) GetMoxfieldSyncState(
	_ context.Context, userID, moxfieldID string,
) (*decks.MoxfieldSyncState, error) {
	m.gotUserID, m.gotMoxfieldID = userID, moxfieldID
	return m.state, m.err
}

func syncedState(changed bool, at *time.Time) *decks.MoxfieldSyncState {
	return &decks.MoxfieldSyncState{
		Deck:         &decks.DeckResponse{ID: "deck-1", Name: "Un deck", Commander: "Atraxa"},
		LastSyncedAt: at,
		Changed:      changed,
	}
}

func TestTriggerSync_Updated(t *testing.T) {
	now := time.Now()
	mock := &mockResyncer{state: syncedState(true, &now)}

	res, err := sync.NewService(mock).TriggerSync(context.Background(), "user-1", "abc123")
	if err != nil {
		t.Fatalf("TriggerSync() error = %v, want nil", err)
	}
	if res.Status != "updated" {
		t.Fatalf("TriggerSync() status = %q, want %q", res.Status, "updated")
	}
	if res.Deck.ID != "deck-1" {
		t.Fatalf("TriggerSync() deck id = %q, want %q", res.Deck.ID, "deck-1")
	}
	if res.LastSyncedAt == nil {
		t.Fatal("TriggerSync() last_synced_at = nil, want el momento del sync")
	}
	if mock.gotUserID != "user-1" || mock.gotMoxfieldID != "abc123" {
		t.Fatalf("TriggerSync() delegó con (%q, %q), want (user-1, abc123)", mock.gotUserID, mock.gotMoxfieldID)
	}
}

func TestTriggerSync_Unchanged(t *testing.T) {
	now := time.Now()
	mock := &mockResyncer{state: syncedState(false, &now)}

	res, err := sync.NewService(mock).TriggerSync(context.Background(), "user-1", "abc123")
	if err != nil {
		t.Fatalf("TriggerSync() error = %v, want nil", err)
	}
	if res.Status != "unchanged" {
		t.Fatalf("TriggerSync() status = %q, want %q", res.Status, "unchanged")
	}
}

// Los errores de dominio de decks se propagan tal cual: el 404 de un deck no
// importado tiene que llegar al handler como decks.ErrDeckNotFound.
func TestTriggerSync_PropagatesDeckError(t *testing.T) {
	mock := &mockResyncer{err: decks.ErrDeckNotFound}

	_, err := sync.NewService(mock).TriggerSync(context.Background(), "user-1", "abc123")
	if !errors.Is(err, decks.ErrDeckNotFound) {
		t.Fatalf("TriggerSync() error = %v, want decks.ErrDeckNotFound", err)
	}
}

func TestGetSyncStatus_Synced(t *testing.T) {
	now := time.Now()
	mock := &mockResyncer{state: syncedState(false, &now)}

	res, err := sync.NewService(mock).GetSyncStatus(context.Background(), "user-1", "abc123")
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v, want nil", err)
	}
	if res.Status != "synced" {
		t.Fatalf("GetSyncStatus() status = %q, want %q", res.Status, "synced")
	}
}

func TestGetSyncStatus_NeverSynced(t *testing.T) {
	mock := &mockResyncer{state: syncedState(false, nil)}

	res, err := sync.NewService(mock).GetSyncStatus(context.Background(), "user-1", "abc123")
	if err != nil {
		t.Fatalf("GetSyncStatus() error = %v, want nil", err)
	}
	if res.Status != "never_synced" {
		t.Fatalf("GetSyncStatus() status = %q, want %q", res.Status, "never_synced")
	}
	if res.LastSyncedAt != nil {
		t.Fatalf("GetSyncStatus() last_synced_at = %v, want nil", *res.LastSyncedAt)
	}
}
