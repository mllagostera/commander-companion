package deckresync_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/deckresync"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

const (
	jobStatusCompleted = "completed"
	jobStatusFailed    = "failed"

	testMoxfieldA = "moxfield-deck-a"
	testMoxfieldB = "moxfield-deck-b"
)

var errSimulatedResyncFailure = errors.New("simulated resync failure")

// fakeDeckLister simulates decks.Service (ListDecks/ResyncFromMoxfield), without
// depending on the real decks module or on Postgres for the decks themselves (the
// jobs do go against real Postgres, via testutil.DB, same as the rest of the project).
type fakeDeckLister struct {
	items   []decks.DeckResponse
	failFor map[string]bool
	delay   time.Duration

	mu    sync.Mutex
	calls []string
}

func (f *fakeDeckLister) ListDecks(
	_ context.Context, _ string, _ common.PageRequest,
) (*decks.DeckListResponse, error) {
	return &decks.DeckListResponse{Items: f.items}, nil
}

func (f *fakeDeckLister) ResyncFromMoxfield(
	_ context.Context, _, moxfieldID string,
) (*decks.MoxfieldSyncState, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	f.calls = append(f.calls, moxfieldID)
	f.mu.Unlock()
	if f.failFor[moxfieldID] {
		return nil, errSimulatedResyncFailure
	}
	return &decks.MoxfieldSyncState{}, nil
}

func newTestSvc(pool *pgxpool.Pool, lister deckresync.DeckLister) deckresync.Service {
	return deckresync.NewService(pool, lister)
}

func truncateResyncTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	testutil.Truncate(t, pool, "deck_resync_jobs", "users")
}

func registerUser(t *testing.T, pool *pgxpool.Pool, email string) *users.UserResponse {
	t.Helper()
	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email, Email: email, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	return user
}

func asFiberError(t *testing.T, err error) *fiber.Error {
	t.Helper()
	var fiberErr *fiber.Error
	if !errors.As(common.MapError(err), &fiberErr) {
		t.Fatalf("error = %v (%T), want *fiber.Error", err, err)
	}
	return fiberErr
}

// waitForTerminalStatus polls GetJobStatus until the job reaches
// completed/failed: the resync runs in its own goroutine, there's no other way to
// synchronize with its completion from the test.
func waitForTerminalStatus(t *testing.T, svc deckresync.Service, userID, jobID string) *deckresync.JobResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, err := svc.GetJobStatus(context.Background(), userID, jobID)
		if err != nil {
			t.Fatalf("GetJobStatus() error = %v", err)
		}
		if job.Status == jobStatusCompleted || job.Status == jobStatusFailed {
			return job
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out esperando que el job llegue a un estado terminal")
	return nil
}

func TestStartResyncAll_NoEligibleDecks_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	user := registerUser(t, pool, "no-decks@example.com")

	// A deck without moxfield_id doesn't count as eligible.
	lister := &fakeDeckLister{items: []decks.DeckResponse{{ID: "d1", MoxfieldID: ""}}}
	svc := newTestSvc(pool, lister)

	_, err := svc.StartResyncAll(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("StartResyncAll() sin decks elegibles: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM deck_resync_jobs WHERE user_id = $1", user.ID,
	).Scan(&count); err != nil {
		t.Fatalf("contando jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("StartResyncAll() sin decks elegibles creó %d jobs, want 0", count)
	}
}

func TestStartResyncAll_Success_CompletesJob(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	user := registerUser(t, pool, "resync-success@example.com")

	lister := &fakeDeckLister{items: []decks.DeckResponse{
		{ID: "d1", MoxfieldID: testMoxfieldA},
		{ID: "d2", MoxfieldID: testMoxfieldB},
		{ID: "d3", MoxfieldID: ""}, // no moxfield_id, shouldn't be counted
	}}
	svc := newTestSvc(pool, lister)

	job, err := svc.StartResyncAll(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartResyncAll() error = %v, want nil", err)
	}
	if job.TotalDecks != 2 {
		t.Fatalf("StartResyncAll() TotalDecks = %d, want 2", job.TotalDecks)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed", final.Status)
	}
	if final.UpdatedCount != 2 || final.FailedCount != 0 {
		t.Fatalf("UpdatedCount/FailedCount = %d/%d, want 2/0", final.UpdatedCount, final.FailedCount)
	}
}

func TestStartResyncAll_PartialFailure_StillCompletes(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	user := registerUser(t, pool, "resync-partial@example.com")

	lister := &fakeDeckLister{
		items:   []decks.DeckResponse{{ID: "d1", MoxfieldID: "deck-ok"}, {ID: "d2", MoxfieldID: "deck-bad"}},
		failFor: map[string]bool{"deck-bad": true},
	}
	svc := newTestSvc(pool, lister)

	job, err := svc.StartResyncAll(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartResyncAll() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed (falla parcial no debería marcar el job entero como failed)", final.Status)
	}
	if final.UpdatedCount != 1 || final.FailedCount != 1 {
		t.Fatalf("UpdatedCount/FailedCount = %d/%d, want 1/1", final.UpdatedCount, final.FailedCount)
	}
}

func TestStartResyncAll_AllDecksFail_StatusFailed(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	user := registerUser(t, pool, "resync-allfail@example.com")

	lister := &fakeDeckLister{
		items:   []decks.DeckResponse{{ID: "d1", MoxfieldID: "deck-bad-1"}, {ID: "d2", MoxfieldID: "deck-bad-2"}},
		failFor: map[string]bool{"deck-bad-1": true, "deck-bad-2": true},
	}
	svc := newTestSvc(pool, lister)

	job, err := svc.StartResyncAll(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartResyncAll() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusFailed {
		t.Fatalf("job.Status = %q, want failed (todos los decks fallaron)", final.Status)
	}
	if final.FailedCount != 2 {
		t.Fatalf("FailedCount = %d, want 2", final.FailedCount)
	}
}

func TestStartResyncAll_AlreadyInProgress_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	user := registerUser(t, pool, "resync-duplicate@example.com")

	// generous delay so the first job stays in_progress when the second
	// StartResyncAll arrives, without depending on tight timing.
	lister := &fakeDeckLister{
		items: []decks.DeckResponse{{ID: "d1", MoxfieldID: testMoxfieldA}, {ID: "d2", MoxfieldID: testMoxfieldB}},
		delay: 500 * time.Millisecond,
	}
	svc := newTestSvc(pool, lister)

	first, err := svc.StartResyncAll(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartResyncAll() primera vez: error = %v, want nil", err)
	}

	_, err = svc.StartResyncAll(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartResyncAll() con resync ya en curso: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}

	// Cleanup: wait for the first job to finish so as not to leave a goroutine
	// writing against a database that the next test is going to truncate.
	waitForTerminalStatus(t, svc, user.ID, first.ID)
}

func TestGetJobStatus_UnknownJob_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	user := registerUser(t, pool, "resync-unknown-job@example.com")

	svc := newTestSvc(pool, &fakeDeckLister{})
	_, err := svc.GetJobStatus(context.Background(), user.ID, "00000000-0000-0000-0000-000000000000")
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetJobStatus() job inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestGetJobStatus_OtherUsersJob_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateResyncTables(t, pool)
	owner := registerUser(t, pool, "resync-owner@example.com")
	other := registerUser(t, pool, "resync-other@example.com")

	lister := &fakeDeckLister{items: []decks.DeckResponse{{ID: "d1", MoxfieldID: testMoxfieldA}}}
	svc := newTestSvc(pool, lister)

	job, err := svc.StartResyncAll(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("StartResyncAll() error = %v, want nil", err)
	}

	_, err = svc.GetJobStatus(context.Background(), other.ID, job.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetJobStatus() job ajeno: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}

	waitForTerminalStatus(t, svc, owner.ID, job.ID)
}
