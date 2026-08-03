package moxfieldimport_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/usuario/commander-companion-backend/internal/common"
	"github.com/usuario/commander-companion-backend/internal/decks"
	"github.com/usuario/commander-companion-backend/internal/moxfield"
	"github.com/usuario/commander-companion-backend/internal/moxfieldimport"
	"github.com/usuario/commander-companion-backend/internal/testutil"
	"github.com/usuario/commander-companion-backend/internal/users"
)

const testPassword = "correct-horse-battery-staple"

const (
	jobStatusPending   = "pending"
	jobStatusCompleted = "completed"
	jobStatusFailed    = "failed"

	testDeckA = "deck-a"
	testDeckB = "deck-b"
)

var errSimulatedImportFailure = errors.New("simulated import failure")

// fakeMoxfieldClient controls what ListDecksByUsername returns, without hitting the
// real API. delay simulates listing latency, to verify StartImport doesn't wait on it.
type fakeMoxfieldClient struct {
	publicIDs []string
	err       error
	delay     time.Duration
}

func (f fakeMoxfieldClient) ListDecksByUsername(_ context.Context, _ string) ([]string, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return f.publicIDs, f.err
}

// fakeDeckImporter simulates decks.Service.ImportFromMoxfield: it fails for the
// publicIDs listed in failFor, reports the ones in alreadyImportedFor as
// decks.ErrDeckAlreadyImported, and optionally delays each call (for the
// duplicate import test, which needs the job to stay in_progress for a while).
type fakeDeckImporter struct {
	failFor            map[string]bool
	alreadyImportedFor map[string]bool
	delay              time.Duration

	mu    sync.Mutex
	calls []string
}

func (f *fakeDeckImporter) ImportFromMoxfield(
	_ context.Context, _ string, req decks.ImportMoxfieldRequest,
) (*decks.DeckResponse, error) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	f.calls = append(f.calls, req.URL)
	f.mu.Unlock()
	if f.alreadyImportedFor[req.URL] {
		return nil, decks.ErrDeckAlreadyImported
	}
	if f.failFor[req.URL] {
		return nil, errSimulatedImportFailure
	}
	return &decks.DeckResponse{}, nil
}

func newTestSvc(
	pool *pgxpool.Pool, mox moxfieldimport.MoxfieldClient, imp moxfieldimport.DeckImporter,
) moxfieldimport.Service {
	return moxfieldimport.NewService(pool, testutil.NewUsersService(pool), imp, mox)
}

func truncateImportTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	testutil.Truncate(t, pool, "moxfield_import_jobs", "users")
}

func registerUserWithMoxfieldUsername(
	t *testing.T, pool *pgxpool.Pool, email, moxfieldUsername string,
) *users.UserResponse {
	t.Helper()
	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "user-" + email, Email: email, Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario de test: %v", err)
	}
	updated, err := usersSvc.UpdateMoxfieldUsername(context.Background(), user.ID, moxfieldUsername)
	if err != nil {
		t.Fatalf("vinculando moxfield_username: %v", err)
	}
	return updated
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
// completed/failed: the import runs in its own goroutine, there's no other way to
// synchronize with its completion from the test.
func waitForTerminalStatus(t *testing.T, svc moxfieldimport.Service, userID, jobID string) *moxfieldimport.JobResponse {
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

func TestStartImport_MoxfieldUpstreamUnavailable_JobFailsAsync(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "upstream-503@example.com", "someone")

	mox := fakeMoxfieldClient{err: moxfield.ErrUpstreamUnavailable}
	svc := newTestSvc(pool, mox, &fakeDeckImporter{})

	// StartImport itself must succeed: listing now happens in the background,
	// so a Moxfield failure surfaces on the job, not on this call.
	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() unexpected error = %v", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusFailed {
		t.Fatalf("job.Status = %q, want %q (moxfield unavailable while listing)", final.Status, jobStatusFailed)
	}
	if final.ErrorMessage == nil || *final.ErrorMessage == "" {
		t.Fatalf("ErrorMessage = %v, want a non-empty message", final.ErrorMessage)
	}
}

// TestStartImport_ReturnsImmediately_DoesNotWaitForMoxfieldListing is the
// regression test for the bug reported against the web client: StartImport
// used to resolve the deck list (a call to Moxfield) synchronously before
// returning, so the request -- and the browser tab that made it -- stayed
// blocked for however long that took. Listing now happens inside the
// background goroutine (see runImport), same as the deck-by-deck import
// already did, so StartImport must return well before a slow listing call finishes.
func TestStartImport_ReturnsImmediately_DoesNotWaitForMoxfieldListing(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "fast-response@example.com", "handle8")

	const listingDelay = 300 * time.Millisecond
	mox := fakeMoxfieldClient{publicIDs: []string{testDeckA}, delay: listingDelay}
	svc := newTestSvc(pool, mox, &fakeDeckImporter{})

	start := time.Now()
	job, err := svc.StartImport(context.Background(), user.ID)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}
	if elapsed >= listingDelay {
		t.Fatalf("StartImport() took %v, want it to return before the %v (background) Moxfield listing call finishes",
			elapsed, listingDelay)
	}
	if job.Status != jobStatusPending {
		t.Fatalf("StartImport() status = %q, want %q (deck list not resolved yet)", job.Status, jobStatusPending)
	}
	if job.TotalDecks != nil {
		t.Fatalf("StartImport() TotalDecks = %v, want nil (not known until listing finishes)", job.TotalDecks)
	}

	waitForTerminalStatus(t, svc, user.ID, job.ID)
}

func TestStartImport_NoMoxfieldUsername_ReturnsBadRequest(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)

	usersSvc := testutil.NewUsersService(pool)
	user, err := usersSvc.RegisterUser(context.Background(), users.RegisterRequest{
		Username: "sin-username", Email: "sin-username@example.com", Password: testPassword,
	})
	if err != nil {
		t.Fatalf("registrando usuario: %v", err)
	}

	svc := newTestSvc(pool, fakeMoxfieldClient{}, &fakeDeckImporter{})
	_, err = svc.StartImport(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusBadRequest {
		t.Fatalf("StartImport() sin moxfield_username: code = %d, want %d", fiberErr.Code, fiber.StatusBadRequest)
	}
}

func TestStartImport_Success_CompletesJob(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "success@example.com", "handle1")

	mox := fakeMoxfieldClient{publicIDs: []string{testDeckA, testDeckB}}
	imp := &fakeDeckImporter{}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}
	if job.Status != jobStatusPending {
		t.Fatalf("StartImport() status = %q, want %q (deck list is resolved in the background)", job.Status, jobStatusPending)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed", final.Status)
	}
	if final.TotalDecks == nil || *final.TotalDecks != 2 {
		t.Fatalf("TotalDecks = %v, want 2", final.TotalDecks)
	}
	if final.ImportedCount != 2 || final.FailedCount != 0 {
		t.Fatalf("ImportedCount/FailedCount = %d/%d, want 2/0", final.ImportedCount, final.FailedCount)
	}
}

func TestStartImport_PartialFailure_StillCompletes(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "partial@example.com", "handle2")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-ok", "deck-bad"}}
	imp := &fakeDeckImporter{failFor: map[string]bool{"deck-bad": true}}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed (falla parcial no debería marcar el job entero como failed)", final.Status)
	}
	if final.ImportedCount != 1 || final.FailedCount != 1 {
		t.Fatalf("ImportedCount/FailedCount = %d/%d, want 1/1", final.ImportedCount, final.FailedCount)
	}
}

// TestStartImport_AlreadyImportedDecks_CountAsImportedNotFailed guards
// against the bug reported after inspecting the DB for duplicate decks:
// re-running the bulk import over decks the user already has must not
// inflate failed_count (they're not failures, decks.ErrDeckAlreadyImported
// means the desired end state -- the deck being in the user's collection --
// already holds).
func TestStartImport_AlreadyImportedDecks_CountAsImportedNotFailed(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "already-imported@example.com", "handle2b")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-new", "deck-had-already"}}
	imp := &fakeDeckImporter{alreadyImportedFor: map[string]bool{"deck-had-already": true}}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusCompleted {
		t.Fatalf("job.Status = %q, want completed", final.Status)
	}
	if final.ImportedCount != 2 || final.FailedCount != 0 {
		t.Fatalf("ImportedCount/FailedCount = %d/%d, want 2/0 (already-imported decks aren't failures)",
			final.ImportedCount, final.FailedCount)
	}
}

func TestStartImport_AllDecksFail_StatusFailed(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "allfail@example.com", "handle3")

	mox := fakeMoxfieldClient{publicIDs: []string{"deck-bad-1", "deck-bad-2"}}
	imp := &fakeDeckImporter{failFor: map[string]bool{"deck-bad-1": true, "deck-bad-2": true}}
	svc := newTestSvc(pool, mox, imp)

	job, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	final := waitForTerminalStatus(t, svc, user.ID, job.ID)
	if final.Status != jobStatusFailed {
		t.Fatalf("job.Status = %q, want failed (todos los decks fallaron)", final.Status)
	}
	if final.FailedCount != 2 {
		t.Fatalf("FailedCount = %d, want 2", final.FailedCount)
	}
}

func TestStartImport_AlreadyInProgress_ReturnsConflict(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "duplicate@example.com", "handle4")

	// generous delay so the first job stays in_progress when the second
	// StartImport arrives, without depending on tight timing.
	mox := fakeMoxfieldClient{publicIDs: []string{testDeckA, testDeckB}}
	imp := &fakeDeckImporter{delay: 500 * time.Millisecond}
	svc := newTestSvc(pool, mox, imp)

	first, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() primera vez: error = %v, want nil", err)
	}

	_, err = svc.StartImport(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusConflict {
		t.Fatalf("StartImport() con import ya en curso: code = %d, want %d", fiberErr.Code, fiber.StatusConflict)
	}

	// Cleanup: wait for the first job to finish so as not to leave a goroutine
	// writing against a database that the next test is going to truncate.
	waitForTerminalStatus(t, svc, user.ID, first.ID)
}

func TestGetJobStatus_UnknownJob_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "unknown-job@example.com", "handle5")

	svc := newTestSvc(pool, fakeMoxfieldClient{}, &fakeDeckImporter{})
	_, err := svc.GetJobStatus(context.Background(), user.ID, "00000000-0000-0000-0000-000000000000")
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetJobStatus() job inexistente: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

func TestGetJobStatus_OtherUsersJob_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	owner := registerUserWithMoxfieldUsername(t, pool, "owner@example.com", "handle6")
	other := registerUserWithMoxfieldUsername(t, pool, "other@example.com", "handle7")

	mox := fakeMoxfieldClient{publicIDs: []string{testDeckA}}
	svc := newTestSvc(pool, mox, &fakeDeckImporter{})

	job, err := svc.StartImport(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	_, err = svc.GetJobStatus(context.Background(), other.ID, job.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetJobStatus() job ajeno: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}

	waitForTerminalStatus(t, svc, owner.ID, job.ID)
}

func TestGetLatestJobStatus_NoImportEver_ReturnsNotFound(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "never-imported@example.com", "handle9")

	svc := newTestSvc(pool, fakeMoxfieldClient{}, &fakeDeckImporter{})
	_, err := svc.GetLatestJobStatus(context.Background(), user.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetLatestJobStatus() sin imports: code = %d, want %d", fiberErr.Code, fiber.StatusNotFound)
	}
}

// TestGetLatestJobStatus_ResumesAcrossPageNavigation is the regression test
// for the bug reported against the web client: settings.vue only kept the
// job ID in a plain ref, so navigating away and back lost track of it and
// the running job looked like it had silently stopped, even though it was
// still going in the background. The frontend now calls this on mount to
// recover the ID it lost.
func TestGetLatestJobStatus_ResumesAcrossPageNavigation(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	user := registerUserWithMoxfieldUsername(t, pool, "resume-tracking@example.com", "handle10")

	mox := fakeMoxfieldClient{publicIDs: []string{testDeckA}}
	svc := newTestSvc(pool, mox, &fakeDeckImporter{})

	started, err := svc.StartImport(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	latest, err := svc.GetLatestJobStatus(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetLatestJobStatus() error = %v, want nil", err)
	}
	if latest.ID != started.ID {
		t.Fatalf("GetLatestJobStatus() ID = %q, want the job just started (%q)", latest.ID, started.ID)
	}

	waitForTerminalStatus(t, svc, user.ID, started.ID)

	// After finishing, it's still "the latest" -- lets the settings page show
	// the completed/failed banner too, not just an in-progress one.
	latest, err = svc.GetLatestJobStatus(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetLatestJobStatus() tras completar: error = %v, want nil", err)
	}
	if latest.Status != jobStatusCompleted {
		t.Fatalf("GetLatestJobStatus() status = %q, want %q", latest.Status, jobStatusCompleted)
	}
}

func TestGetLatestJobStatus_OtherUsersJob_NotVisible(t *testing.T) {
	pool := testutil.DB(t)
	truncateImportTables(t, pool)
	owner := registerUserWithMoxfieldUsername(t, pool, "latest-owner@example.com", "handle11")
	other := registerUserWithMoxfieldUsername(t, pool, "latest-other@example.com", "handle12")

	mox := fakeMoxfieldClient{publicIDs: []string{testDeckA}}
	svc := newTestSvc(pool, mox, &fakeDeckImporter{})

	job, err := svc.StartImport(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("StartImport() error = %v, want nil", err)
	}

	_, err = svc.GetLatestJobStatus(context.Background(), other.ID)
	if fiberErr := asFiberError(t, err); fiberErr.Code != fiber.StatusNotFound {
		t.Fatalf("GetLatestJobStatus() de otro usuario sin imports propios: code = %d, want %d",
			fiberErr.Code, fiber.StatusNotFound)
	}

	waitForTerminalStatus(t, svc, owner.ID, job.ID)
}
